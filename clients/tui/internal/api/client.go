// Package api provides an OpenAI-compatible streaming chat client.
//
// It discovers model capabilities dynamically instead of hardcoding:
//  1. Queries the standard OpenAI /v1/models endpoint
//  2. If metadata is sparse, tries LM Studio's /api/v1/models
//  3. Uses discovered info to drive API calls
//
// All config comes from env vars (controlled by run-tui.ps1):
//
//	EXTENDAI_BASE_URL  — full API base URL (e.g. http://host:port/v1)
//	EXTENDAI_API_KEY   — API key (optional)
//	EXTENDAI_MODEL     — Model ID (optional; if empty, uses first model)
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── Types ───────────────────────────────────────────────────

// Message represents a single chat message in the request.
// Supports text, tool_calls (assistant), and tool_result (user).
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"` // Always include content (required by most APIs)
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"` // for tool role
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call within a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ModelInfo holds discovered capabilities for the active model.
type ModelInfo struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name,omitempty"`
	ContextLength   int    `json:"context_length,omitempty"`
	SupportsVision  bool   `json:"supports_vision,omitempty"`
	SupportsTools   bool   `json:"supports_tools,omitempty"`
	SupportsReasoning bool `json:"supports_reasoning,omitempty"`
	Provider        string `json:"provider,omitempty"` // "openai", "lmstudio", "ollama", etc.
}

// StreamEvent is yielded on the channel returned by SendMessage.
type StreamEvent struct {
	Content          string     // regular content delta
	ReasoningContent string     // reasoning/thinking delta (shown dimmer)
	ToolCalls        []ToolCall // tool calls (only in final chunk)
	Done             bool
	Error            error
	Usage            *UsageInfo // token usage (only in final chunk)
}

// UsageInfo holds token usage from the API response.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// capReasoning holds LM Studio's reasoning field shape.
type capReasoning struct {
	Allowed []string `json:"allowed_options"`
	Default string   `json:"default"`
}

// lmStudioInstance holds one loaded model instance from LM Studio's API.
type lmStudioInstance struct {
	ID     string `json:"id"`
	Config struct {
		ContextLength int  `json:"context_length"`
	} `json:"config"`
}

// lmStudioModel is one model entry from LM Studio's /api/v1/models.
type lmStudioModel struct {
	Key             string             `json:"key"`
	DisplayName     string             `json:"display_name"`
	MaxContext      int                `json:"max_context_length"`
	LoadedInstances []lmStudioInstance `json:"loaded_instances"`
	Capabilities    struct {
		Vision          *bool          `json:"vision"`
		TrainedForTools *bool          `json:"trained_for_tool_use"`
		Reasoning       *capReasoning  `json:"reasoning"`
	} `json:"capabilities"`
}

// ─── Client ──────────────────────────────────────────────────

// Client is an auto-discovering streaming chat client.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	info    *ModelInfo // discovered capabilities
	history []Message
	http    *http.Client

	// Thinking intensity (reasoning effort)
	// "low" | "medium" | "high" | "" (default/off)
	ReasoningEffort string
}

// NewFromEnv creates a client from environment variables.
// Returns nil if EXTENDAI_BASE_URL is not set.
func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("EXTENDAI_BASE_URL"), "/")
	if baseURL == "" {
		return nil
	}
	// Use Transport with no overall timeout for streaming.
	// The Timeout on http.Client kills streaming connections.
	return &Client{
		baseURL: baseURL,
		apiKey:  os.Getenv("EXTENDAI_API_KEY"),
		model:   os.Getenv("EXTENDAI_MODEL"),
		http: &http.Client{
			Timeout: 0, // no overall timeout — streaming needs open-ended body read
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second, // max wait for first byte
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
}

// ─── Discovery ───────────────────────────────────────────────

// Discover queries the provider's model endpoints and populates model capabilities.
// Returns the first error if discovery fails entirely.
func (c *Client) Discover() error {
	// 1) Try standard OpenAI /v1/models
	info, err := c.discoverOpenAI()
	if err == nil && info != nil {
		c.info = info
		return nil
	}

	// 2) Fall back to LM Studio /api/v1/models
	info, err = c.discoverLMStudio()
	if err == nil && info != nil {
		c.info = info
		return nil
	}

	return fmt.Errorf("api: discovery failed (standard + lmstudio): no model info available")
}

// discoverOpenAI queries GET /v1/models and returns a ModelInfo.
// OpenAI's standard response only has id/object/owned_by, so we enrich
// with sensible defaults.
func (c *Client) discoverOpenAI() (*ModelInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /v1/models: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Object   string `json:"object"`
			OwnedBy  string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Pick the target model
	target := c.model
	if target == "" && len(result.Data) > 0 {
		target = result.Data[0].ID
		c.model = target
	}

	if target == "" {
		return nil, fmt.Errorf("no models available")
	}

	// Basic info — most providers return minimal data here.
	// richer metadata will come from LM Studio discovery if needed.
	return &ModelInfo{
		ID:      target,
		Provider: "openai-compat",
	}, nil
}

// discoverLMStudio queries GET /api/v1/models for LM Studio's rich metadata.
func (c *Client) discoverLMStudio() (*ModelInfo, error) {
	// Build LM Studio URL by replacing the last path segment (v1 → api/v1)
	lmURL := c.buildLMStudioURL()
	if lmURL == "" {
		return nil, fmt.Errorf("cannot build LM Studio URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lmURL+"/api/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /api/v1/models: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Models []lmStudioModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Find target model
	target := c.model
	for _, m := range result.Models {
		if m.Key == target || target == "" {
			if target == "" {
				target = m.Key
				c.model = target
			}
			info := &ModelInfo{
				ID:          target,
				DisplayName: m.DisplayName,
				ContextLength: m.MaxContext,
				Provider:    "lmstudio",
			}

			// Use loaded instance context_length if available (more precise)
			if m.MaxContext == 0 && len(m.LoadedInstances) > 0 {
				info.ContextLength = m.LoadedInstances[0].Config.ContextLength
			}

			// Capabilities
			if m.Capabilities.Vision != nil {
				info.SupportsVision = *m.Capabilities.Vision
			}
			if m.Capabilities.TrainedForTools != nil {
				info.SupportsTools = *m.Capabilities.TrainedForTools
			}
			if m.Capabilities.Reasoning != nil && len(m.Capabilities.Reasoning.Allowed) > 0 {
				info.SupportsReasoning = true
			}

			return info, nil
		}
	}

	return nil, fmt.Errorf("model %q not found in LM Studio", target)
}

// buildLMStudioURL converts a standard OpenAI URL to LM Studio URL.
// e.g. "http://host:1234/v1" → "http://host:1234"
func (c *Client) buildLMStudioURL() string {
	// If URL ends with /v1, strip it
	u := strings.TrimSuffix(c.baseURL, "/v1")
	u = strings.TrimSuffix(u, "/v1/")
	if u == c.baseURL && u != "" {
		// Not an OpenAI-style URL — use as-is
		return u
	}
	return u
}

// ─── Accessors ───────────────────────────────────────────────

// Model returns the configured (or discovered) model ID.
func (c *Client) Model() string { return c.model }

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// Info returns the discovered model capabilities (nil before Discover).
func (c *Client) Info() *ModelInfo { return c.info }

// Reset clears conversation history (keeps system prompt).
func (c *Client) Reset() {
	c.history = []Message{
		{Role: "system", Content: "You are a helpful AI assistant."},
	}
}

// SetHistory replaces the conversation history with the given messages.
// Used to sync with session messages on startup.
func (c *Client) SetHistory(messages []Message) {
	c.history = messages
}

// GetHistory returns the current conversation history.
func (c *Client) GetHistory() []Message {
	return c.history
}

// ─── Chat ────────────────────────────────────────────────────

// SendMessage sends a user message and returns a channel of stream events.
// The caller reads until Done or Error.
// tools is an optional list of tool definitions to include in the request.
func (c *Client) SendMessage(ctx context.Context, content string, tools []ToolDefinition) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 64)

	// Ensure content is never empty
	if content == "" {
		content = "(no content)"
	}

	// Append user message
	c.history = append(c.history, Message{Role: "user", Content: content})

	// Build request body
	body := map[string]interface{}{
		"model":    c.model,
		"messages": c.history,
		"stream":   true,
	}

	// Include tools if provided
	if len(tools) > 0 {
		body["tools"] = tools
	}

	// If the model supports reasoning, include the reasoning parameter
	if c.info != nil && c.info.SupportsReasoning {
		effort := c.ReasoningEffort
		if effort == "" {
			effort = "medium"
		}
		body["reasoning"] = map[string]string{"effort": effort}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("api: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("api: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	go c.stream(req, ch)
	return ch, nil
}

// stream performs the SSE request and feeds events into ch.
func (c *Client) stream(req *http.Request, ch chan<- StreamEvent) {
	defer close(ch)

	resp, err := c.http.Do(req)
	if err != nil {
		ch <- StreamEvent{Error: fmt.Errorf("api: request failed: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ch <- StreamEvent{Error: fmt.Errorf("api: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))}
		return
	}

	// Wrap body with a deadline for the first byte — prevents blocking forever
	// if the server accepts the connection but never sends data.
	deadlineReader := newDeadlineReader(resp.Body, 60*time.Second)
	defer deadlineReader.Stop()

	scanner := bufio.NewScanner(deadlineReader)
	scanner.Buffer(make([]byte, 0, 65536), 65536)

	var fullContent strings.Builder
	var fullReasoning strings.Builder
	var lastUsage *UsageInfo
	var toolCalls []ToolCall
	toolCallMap := make(map[int]*ToolCall) // index → accumulated tool call

streamLoop:
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string     `json:"content"`
					ReasoningContent string     `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *UsageInfo `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Capture usage if present
		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		// Handle tool_calls accumulation
		for _, tc := range chunk.Choices[0].Delta.ToolCalls {
			existing, ok := toolCallMap[tc.Index]
			if !ok {
				// New tool call
				newTC := ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
				toolCallMap[tc.Index] = &newTC
			} else {
				// Accumulate arguments
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Type != "" {
					existing.Type = tc.Type
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			}
		}

		// Read content and reasoning
		delta := chunk.Choices[0].Delta.Content
		rc := chunk.Choices[0].Delta.ReasoningContent

		if delta == "" && rc == "" {
			if chunk.Choices[0].FinishReason != nil {
				break streamLoop
			}
			continue
		}

		// Send both content and reasoning_content as separate events
		if delta != "" {
			fullContent.WriteString(delta)
			ch <- StreamEvent{Content: delta}
		}
		if rc != "" {
			fullReasoning.WriteString(rc)
			ch <- StreamEvent{ReasoningContent: rc}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Error: fmt.Errorf("api: read stream: %w", err)}
		return
	}

	// Collect accumulated tool calls
	for i := 0; i < len(toolCallMap); i++ {
		if tc, ok := toolCallMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}

	// Store assistant response (with tool calls if present)
	assistantMsg := Message{
		Role:      "assistant",
		Content:   fullContent.String(),
		ToolCalls: toolCalls,
	}
	c.history = append(c.history, assistantMsg)

	// If no usage from API, estimate from content length
	if lastUsage == nil {
		totalOutput := fullContent.Len() + fullReasoning.Len()
		totalInput := 0
		for _, m := range c.history {
			totalInput += len(m.Content)
		}
		lastUsage = &UsageInfo{
			PromptTokens:     totalInput / 4,
			CompletionTokens: totalOutput / 4,
			TotalTokens:      (totalInput + totalOutput) / 4,
		}
	}

	ch <- StreamEvent{Done: true, Usage: lastUsage, ToolCalls: toolCalls}
}

// AddToolResult adds a tool result message to the history.
// This is used to feed tool execution results back to the model.
func (c *Client) AddToolResult(toolCallID string, content string, isError bool) {
	role := "tool"
	resultContent := content
	if isError {
		resultContent = "Error: " + content
	}
	// Ensure content is never empty (some APIs require non-empty content)
	if resultContent == "" {
		resultContent = "(empty result)"
	}
	c.history = append(c.history, Message{
		Role:       role,
		Content:    resultContent,
		ToolCallID: toolCallID,
	})
}

// ─── Deadline reader (prevents blocking on slow servers) ───────

// deadlineReader wraps an io.ReadCloser and closes it after a timeout
// if no data has been read. This prevents the scanner from blocking forever
// when the server accepts the connection but never sends data.
type deadlineReader struct {
	r       io.ReadCloser
	timer   *time.Timer
	once    sync.Once
	timeout time.Duration
}

func newDeadlineReader(r io.ReadCloser, timeout time.Duration) *deadlineReader {
	dr := &deadlineReader{r: r, timeout: timeout}
	dr.resetTimer()
	return dr
}

func (dr *deadlineReader) resetTimer() {
	if dr.timer != nil {
		dr.timer.Stop()
	}
	dr.timer = time.AfterFunc(dr.timeout, func() {
		dr.once.Do(func() {
			dr.r.Close()
		})
	})
}

func (dr *deadlineReader) Read(p []byte) (int, error) {
	n, err := dr.r.Read(p)
	if n > 0 {
		dr.resetTimer() // reset deadline on progress
	}
	return n, err
}

func (dr *deadlineReader) Close() error {
	if dr.timer != nil {
		dr.timer.Stop()
	}
	return dr.r.Close()
}

func (dr *deadlineReader) Stop() {
	if dr.timer != nil {
		dr.timer.Stop()
	}
}
