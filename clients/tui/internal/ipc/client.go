package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
)

// Client communicates with the TS server via stdio JSON-Lines protocol.
type Client struct {
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	cmd    *exec.Cmd
}

// NewClient spawns the TS server subprocess and establishes IPC.
// The TS server must accept --tui-mode and speak JSON-Lines protocol.
func NewClient() (*Client, error) {
	// Try to spawn the TS server process
	cmd := exec.Command("node", []string{
		"dist/cli/src/index.js",
		"--tui-mode",
	}...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start server: %w", err)
	}

	return &Client{
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		cmd:    cmd,
	}, nil
}

// SendMessage sends a user chat message to the TS server.
func (c *Client) SendMessage(content string) error {
	msg := map[string]string{
		"type":    "message",
		"content": content,
	}
	return c.writeJSON(msg)
}

// ReadMessage reads one JSON packet from stdout.
// Returns a discriminated union: either *protocol.RenderCommand (type="render") or *Response (type="message_done"/"error").
func (c *Client) ReadMessage() (interface{}, error) {
	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return nil, io.EOF
	}

	data := c.stdout.Bytes()

	// First unmarshal into envelope to read the type field
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}

	switch envelope.Type {
	case "render":
		var cmd protocol.RenderCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return nil, fmt.Errorf("parse render command: %w", err)
		}
		return &cmd, nil

	case "message_done", "message_chunk":
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		return &resp, nil

	default:
		return nil, fmt.Errorf("unknown message type: %s", envelope.Type)
	}
}

// writeJSON marshals v and writes it as one JSON line to stdin.
func (c *Client) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// Close terminates the IPC connection.
func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

// Response from TS server for streaming content.
type Response struct {
	Content string `json:"content"`
	Done    bool   `json:"done,omitempty"`
	Error   string `json:"error,omitempty"`
}
