package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/api"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/ipc"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/model"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
)



func main() {
	// Detect terminal size
	theme.InitTerm()

	// Try to spawn the TS server
	ipcClient, _ := tryStartIPC()

	// If no TS server, try standalone API client (LM Studio / OpenAI-compatible)
	var aiClient *api.Client
	if ipcClient == nil {
		aiClient = api.NewFromEnv()
		if aiClient != nil {
			// Discover model capabilities from provider
			if err := aiClient.Discover(); err != nil {
				fmt.Fprintf(os.Stderr, "API discovery warning: %v\n", err)
			}
		}
	}

	m := model.New(ipcClient, aiClient)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())

	if ipcClient != nil {
		// IPC reader goroutine: forward TS messages to UI
		go func() {
			p.Send(model.MsgServerStatus{Connected: true})

			for {
				msg, err := ipcClient.ReadMessage()
				if err != nil {
					// EOF or connection error — server disconnected gracefully
					p.Send(model.MsgServerStatus{Connected: false, Error: "Connection closed"})
					break
				}

				switch v := msg.(type) {
				case *ipc.Response:
					if v.Error != "" {
						p.Send(model.MsgError{Err: v.Error})
					} else if v.Done {
						p.Send(model.MsgRenderCommand{
							Command: &protocol.RenderCommand{
								Components: []protocol.Component{
									{
										Type: "message",
										Props: map[string]interface{}{
											"role":    "assistant",
											"content": v.Content,
										},
									},
								},
							},
						})
					} else {
						p.Send(model.MsgStreamChunk{
							Content: v.Content,
							Done:    v.Done,
							Error:   v.Error,
						})
					}
				case *protocol.RenderCommand:
					p.Send(model.MsgRenderCommand{Command: v})
				case *protocol.AIStatusMsg:
					p.Send(model.MsgAIStatus{
						Status: v.Status,
						Model:  v.Model,
						Label:  v.Label,
					})
				case *protocol.StreamChunk:
					p.Send(model.MsgStreamChunk{
						Content: v.Content,
						Done:    v.Done,
						Error:   v.Error,
					})
				case *protocol.StatusUpdate:
					p.Send(model.MsgStatusUpdate{Update: *v})
				}
			}
		}()
	} else if aiClient != nil {
		// Standalone mode with API client (LM Studio, etc.)
		go func() {
			// Build capability summary
			info := aiClient.Info()
			capStr := ""
			if info != nil {
				var caps []string
				if info.ContextLength > 0 {
					caps = append(caps, fmt.Sprintf("%dK ctx", info.ContextLength/1024))
				}
				if info.SupportsVision {
					caps = append(caps, "vision")
				}
				if info.SupportsTools {
					caps = append(caps, "tools")
				}
				if info.SupportsReasoning {
					caps = append(caps, "reasoning")
				}
				if len(caps) > 0 {
					capStr = "\nCapabilities: " + strings.Join(caps, ", ")
				}
			}

			p.Send(model.MsgServerStatus{
				Connected: true,
			})
			p.Send(model.MsgRenderCommand{
				Command: &protocol.RenderCommand{
					Components: []protocol.Component{
						{
							Type: "message",
							Props: map[string]interface{}{
								"role":    "system",
								"content": "ExtendAI Lab TUI — standalone\nAPI: " + aiClient.BaseURL() + "\nModel: " + aiClient.Model() + capStr + "\nType /help for commands.",
							},
						},
					},
				},
			})
		}()
	} else {
		// Standalone mode — no TS server, no API client.
		go func() {
			p.Send(model.MsgServerStatus{Connected: false, Error: "No provider configured"})
			p.Send(model.MsgRenderCommand{
				Command: &protocol.RenderCommand{
					Components: []protocol.Component{
						{
							Type: "message",
							Props: map[string]interface{}{
								"role":    "system",
								"content": "ExtendAI Lab TUI — no provider\nSet EXTENDAI_BASE_URL, EXTENDAI_API_KEY, and EXTENDAI_MODEL\nor run with the full TS server.\nType /help for commands.",
							},
						},
					},
				},
			})
		}()
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if ipcClient != nil {
		ipcClient.Close()
	}
}

// tryStartIPC attempts to spawn the TS server subprocess.
// Returns nil gracefully when the server binary doesn't exist or can't start.
func tryStartIPC() (*ipc.Client, error) {
	// First check if the dist file exists
	if _, err := os.Stat("dist/cli/src/index.js"); os.IsNotExist(err) {
		return nil, fmt.Errorf("dist/cli/src/index.js not found")
	}

	client, err := ipc.NewClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}
