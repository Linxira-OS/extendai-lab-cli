package main

import (
	"fmt"
	"os"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/ipc"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/model"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/protocol"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Detect terminal size
	theme.InitTerm()

	var ipcClient *ipc.Client

	// Try to spawn the TS server
	client, err := ipc.NewClient()
	if err != nil {
		// No server available — standalone mode
		ipcClient = nil
	} else {
		ipcClient = client
	}

	m := model.New(ipcClient)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if ipcClient != nil {
		// IPC reader goroutine: forward TS messages to UI
		go func() {
			p.Send(model.MsgServerStatus{Connected: true})

			for {
				msg, err := ipcClient.ReadMessage()
				if err != nil {
					p.Send(model.MsgError{Err: err.Error()})
					break
				}

				switch v := msg.(type) {
				case *ipc.Response:
					// Legacy streaming response
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
				case *protocol.RenderCommand:
					p.Send(model.MsgRenderCommand{Command: v})
				}
			}
		}()
	} else {
		p.Send(model.MsgServerStatus{Connected: false, Error: "TS server not found. Run standalone mode."})
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if ipcClient != nil {
		ipcClient.Close()
	}
}
