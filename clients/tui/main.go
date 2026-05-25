package main

import (
	"fmt"
	"os"

	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/ipc"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/model"
	"github.com/Linxira-OS/extendai-lab-cli/clients/tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Detect terminal size
	theme.InitTerm()

	m := model.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	go func() {
		// Connect to TS server via stdio IPC
		client, err := ipc.NewClient()
		if err != nil {
			// No server available — standalone mode
			p.Send(model.MsgServerStatus{Connected: false, Error: err.Error()})
			return
		}
		p.Send(model.MsgServerStatus{Connected: true})

		// Read responses from server and forward to UI
		for {
			resp, err := client.ReadResponse()
			if err != nil {
				p.Send(model.MsgError{Err: err.Error()})
				break
			}
			p.Send(model.MsgStreamChunk{Content: resp.Content, Done: resp.Done})
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
