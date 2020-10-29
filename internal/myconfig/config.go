package myconfig

import "crypto/tls"

// SharedConfig will hold the objects the web server
// and the socket server have in common
type SharedConfig struct {
	TLS          bool
	SelfSigned   bool
	Key          string
	Cert         string
	Pass         string
	TLSConfig    *tls.Config
	GoshsVersion string
}
