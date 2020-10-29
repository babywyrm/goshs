package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"

	"github.com/patrickhener/goshs/internal/myconfig"
	"github.com/patrickhener/goshs/internal/myhttp"
	"github.com/patrickhener/goshs/internal/mysock"
)

const goshsVersion = "v0.0.6"

var help = `
  Usage: goshs [command] [--help]

  Version: ` + goshsVersion + ` (` + runtime.Version() + `)

  Commands:
	server - runs goshs in server mode
	client - runs goshs in client mode (TUI)

  Read more:
	https://github.com/patrickhener/goshs


`

func main() {
	// // Random Seed generation (used for CA serial)
	// rand.Seed(time.Now().UnixNano())
	// // Setup the custom file server
	// server := &myhttp.WebServer{
	// 	IP:         ip,
	// 	Port:       port,
	// 	Webroot:    webroot,
	// 	SSL:        ssl,
	// 	SelfSigned: selfsigned,
	// 	MyCert:     myCert,
	// 	MyKey:      myKey,
	// 	BasicAuth:  basicAuth,
	// 	Version:    goshsVersion,
	// }
	// server.Start()

	version := flag.Bool("version", false, "")
	v := flag.Bool("v", false, "")
	flag.Bool("help", false, "")
	flag.Bool("h", false, "")
	flag.Usage = func() {}
	flag.Parse()

	if *version || *v {
		fmt.Printf("goshs version is %+v\n", goshsVersion)
		os.Exit(0)
	}

	args := flag.Args()

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "server":
		server(args)
	case "client":
		client(args)
	default:
		fmt.Fprintf(os.Stderr, help)
		os.Exit(0)
	}

}

var commonHelp = `
  Version:
  ` + goshsVersion + ` (` + runtime.Version() + `)

  Read more:
	https://github.com/patrickhener/goshs


`

var serverHelp = `
  Usage: goshs server [options]

  Web server options:

    This options will apply to the frontend provided by goshs

    --webip, -wip, The ip the web server listens on
    (defaults to 0.0.0.0)

    --webport, -wp, The port the web server listens on
    (defaults to 8000)

    --dir, -d, The root directory of the web server to serve
    (defaults to the current working directory)

  TLS options:

    This options will apply to the frontend and the api
    (if activated) provided by goshs

    --tls, -t, Activate Transport Layer Security (HTTPS)
    (defaults to false), You will have to provide either self-signed or key and cert

    --self-signed, -ss, Use automatically generated self-signed certificate
    (defaults to false), Goes with --tls

    --server-key, -sk, Use specific server key [path]
    (defaults to none), Goes with --tls and --server-cert

    --server-cert, -sc, Use specific server cert [path]
    (defaults to none), Goes with --tls and --server-cert

  Authentication options:

    This options will apply to the frontend and the api
    (if activated) provided by goshs

    --pass, -p, Password to authenticate with
    (defaults to none, default user would be: gopher)
    Used as Basic Authentication at the frontend
	User as Authentication Password for the TUI

  Socket server options:

    This options will activate the socket server listening for the TUI client
    and is therefore mandatory if you want to use the TUI client.

    The socket server will take TLS and Authentication options from the web
    server options

    --sock, -s, Activate the socket server
    (defaults to false)

    --sockip, -sip, The ip the socket server listens on
    (defaults to 0.0.0.0)

    --socketport, -sp, The port the socket server listens on
    (defaults to 8001)

  Usage Examples:

     TODO Examples here

` + commonHelp

func server(args []string) {
	var sockserver mysock.SockServer

	flags := flag.NewFlagSet("server", flag.ContinueOnError)

	wip := flags.String("wip", "", "")
	wp := flags.Int("wp", 0, "")
	d := flags.String("d", "", "")
	t := flags.Bool("t", false, "")
	ss := flags.Bool("ss", false, "")
	sk := flags.String("sk", "", "")
	sc := flags.String("sc", "", "")
	p := flags.String("p", "", "")
	s := flags.Bool("s", false, "")
	sip := flags.String("sip", "", "")
	sp := flags.Int("sp", 0, "")

	webip := flags.String("webip", "", "")
	webport := flags.Int("webport", 0, "")
	dir := flags.String("dir", "", "")
	tls := flags.Bool("tls", false, "")
	selfsigned := flags.Bool("self-signed", false, "")
	serverkey := flags.String("server-key", "", "")
	servercert := flags.String("server-cert", "", "")
	pass := flags.String("pass", "", "")
	sock := flags.Bool("sock", false, "")
	sockip := flags.String("sockip", "", "")
	sockport := flags.Int("sockport", 0, "")

	flags.Usage = func() {
		fmt.Print(serverHelp)
		os.Exit(0)
	}
	flags.Parse(args)

	if *webip == "" {
		*webip = *wip
	}

	if *webip == "" {
		*webip = "0.0.0.0"
	}

	if *webport == 0 {
		*webport = *wp
	}

	if *webport == 0 {
		*webport = 8000
	}

	if *dir == "" {
		*dir = *d
	}

	if *dir == "" {
		*dir, _ = os.Getwd()
	}

	if !*tls {
		*tls = *t
	}

	if !*selfsigned {
		*selfsigned = *ss
	}

	if *serverkey == "" {
		*serverkey = *sk
	}

	if *servercert == "" {
		*servercert = *sc
	}

	if *pass == "" {
		*pass = *p
	}

	if !*sock {
		*sock = *s
	}

	if *sockip == "" {
		*sockip = *sip
	}

	if *sockip == "" {
		*sockip = "0.0.0.0"
	}

	if *sockport == 0 {
		*sockport = *sp
	}

	if *sockport == 0 {
		*sockport = 8001
	}

	shared := myconfig.SharedConfig{
		TLS:          *tls,
		SelfSigned:   *selfsigned,
		Key:          *serverkey,
		Cert:         *servercert,
		Pass:         *pass,
		GoshsVersion: goshsVersion,
	}

	webserver := myhttp.WebServer{
		IP:           *webip,
		Port:         *webport,
		Webroot:      *dir,
		SharedConfig: &shared,
	}

	go func() {
		err := webserver.Start()
		if err != http.ErrServerClosed {
			log.Fatalf("ERROR: Error starting web server: %+v", err)
		}
	}()

	if *sock {
		sockserver = mysock.SockServer{
			IP:           *sockip,
			Port:         *sockport,
			SharedConfig: &shared,
		}

		go func() {
			if err := sockserver.Start(); err != nil {
				log.Fatalf("ERROR: Error starting socket server: %+v", err)
			}
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 15)
	defer cancel()

	// Stop services
	webserver.Stop(ctx)
	if *sock {
		sockserver.Stop()
	}

	// Output
	log.Println("Got ctrl+c, shutting down ...")
	os.Exit(0)

}

var clientHelp = `
  Usage: goshs client [options]


` + commonHelp

func client(args []string) {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)

	flags.Usage = func() {
		fmt.Print(clientHelp)
		os.Exit(0)
	}
	flags.Parse(args)

	sockclient := mysock.SockClient{
		ServerIP:   "127.0.0.1",
		ServerPort: 8001,
		Pass:       "",
	}

	sockclient.Start()
}
