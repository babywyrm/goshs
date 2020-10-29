package mysock

import (
	"fmt"

	"github.com/patrickhener/goshs/internal/myconfig"
)

// SockServer holds the socket server information
type SockServer struct {
	IP           string
	Port         int
	SharedConfig *myconfig.SharedConfig
}

// Start will launch the socket server
func (ss *SockServer) Start() error {
	fmt.Println("DEBUG: Starting socket server")
	fmt.Printf("DEBUG: Socket Server IP is: %+v\n", ss.IP)
	fmt.Printf("DEBUG: Socket Server Port is: %+v\n", ss.Port)
	fmt.Printf("DEBUG: Socket Server TLS is: %+v\n", ss.SharedConfig.TLS)
	fmt.Printf("DEBUG: Socket Server Key is: %+v\n", ss.SharedConfig.Key)
	fmt.Printf("DEBUG: Socket Server Cert is: %+v\n", ss.SharedConfig.Cert)
	return nil
}

// Stop will gracefully shutdown the socket server
func (ss *SockServer) Stop() {
	fmt.Println("DEBUG: Shutting down socket server")
}
