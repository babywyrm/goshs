package mysock

import "github.com/patrickhener/goshs/internal/mytui"

// SockClient will hold all informations of the socket client
type SockClient struct {
	ServerIP   string
	ServerPort int
	Pass       string
}

// Start will start the socket client
// After connecting successfully it will display the TUI
func (sc *SockClient) Start() {
	mytui.DisplayTUI()
}
