package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// Response from TS server
type Response struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// Client communicates with the TS server via stdio
type Client struct {
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	cmd    *exec.Cmd
}

func NewClient() (*Client, error) {
	// Try to spawn the TS server process
	// For now, look for it in the project dist
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

func (c *Client) SendMessage(content string) error {
	msg := map[string]string{
		"type":    "message",
		"content": content,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

func (c *Client) ReadResponse() (*Response, error) {
	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return nil, io.EOF
	}

	var resp Response
	if err := json.Unmarshal([]byte(c.stdout.Text()), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}
