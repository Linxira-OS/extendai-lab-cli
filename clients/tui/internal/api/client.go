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
	"time"
)

// ─── Types ───────────────────────────────────────────────────

// Message represents a single chat message in the request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	Content          string // regular content delta
	ReasoningContent string // reasoning/thinking delta (shown dimmer)
	Done             bool
	Error            error
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
}

// NewFromEnv creates a client from environment variables.
// Returns nil if EXTENDAI_BASE_URL is not set.
func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("EXTENDAI_BASE_URL"), "/")
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  os.Getenv("EXTENDAI_API_KEY"),
		model:   os.Getenv("EXTENDAI_MODEL"),
		http:    &http.Client{Timeout: 5 * time.Minute},
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

// ─── Chat ────────────────────────────────────────────────────

// SendMessage sends a user message and returns a channel of stream events.
// The caller reads until Done or Error.
func (c *Client) SendMessage(ctx context.Context, content string) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 64)

	// Append user message
	c.history = append(c.history, Message{Role: "user", Content: content})

	// Build request body — use reasoning if model supports it
	body := map[string]interface{}{
		"model":    c.model,
		"messages": c.history,
		"stream":   true,
	}

	// If the model supports reasoning, include the reasoning parameter
	if c.info != nil && c.info.SupportsReasoning {
		body["reasoning"] = map[string]string{"mode": "on"}
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

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 65536), 65536)

	var fullContent strings.Builder

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
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		// Read both content and reasoning_content
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
			ch <- StreamEvent{ReasoningContent: rc}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Error: fmt.Errorf("api: read stream: %w", err)}
		return
	}

	// Store assistant response
	assistantMsg := fullContent.String()
	c.history = append(c.history, Message{Role: "assistant", Content: assistantMsg})

	ch <- StreamEvent{Done: true}
}
