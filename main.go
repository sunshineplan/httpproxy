package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sunshineplan/service"
	"github.com/sunshineplan/utils/flags"
	"github.com/sunshineplan/utils/httpsvr"
)

// common
var (
	mode      = flag.String("mode", "server", "server or client mode")
	accesslog = flag.String("access-log", "", "Path to access log file")
	errorlog  = flag.String("error-log", "", "Path to error log file")
	debug     = flag.Bool("debug", false, "debug")
)

const commonFlag = `
common:
  --host <string>
    	Listening host
  --port <number>
    	Listening port
  --mode <string>
    	Specify server or client mode (default: server)
  --access-log <file>
    	Path to access log file
  --error-log <file>
    	Path to error log file
  --update <url>
    	Update URL
`

// server
var (
	https     = flag.Bool("https", false, "Serve as HTTPS proxy server")
	cert      = flag.String("cert", "", "Path to certificate file")
	privkey   = flag.String("privkey", "", "Path to private key file")
	secrets   = flag.String("secrets", "", "Path to secrets file for Basic Authentication")
	whitelist = flag.String("whitelist", "", "Path to whitelist file")
	status    = flag.String("status", "", "Path to status file")
	keep      = flag.Int("keep", 100, "Count of status files")
)

const serverFlag = `
server side:
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
  --secrets <file>
    	Path to secrets file for Basic Authentication
  --whitelist <file>
    	Path to whitelist file
  --status <file>
    	Path to status file
  --keep number
    	Count of status files (default: 100)
`

// client
var (
	proxy    = flag.String("proxy", "", "Proxy address")
	username = flag.String("username", "", "Username")
	password = flag.String("password", "", "Password")
)

const clientFlag = `
client side:
  --proxy <string>
    	Proxy address
  --username <string>
    	Username for Basic Authentication
  --password <string>
    	Password for Basic Authentication
`

var (
	self string

	server = httpsvr.New()
	svc    = service.New()
)

func init() {
	var err error
	self, err = os.Executable()
	if err != nil {
		log.Fatalln("Failed to get self path:", err)
	}
	svc.Name = "HTTPProxy"
	svc.Desc = "HTTP(S) Proxy Server"
	svc.Exec = run
	svc.TestExec = test
	svc.Options = service.Options{
		Dependencies: []string{"After=network.target"},
		Others:       []string{"ExecReload=kill -HUP $MAINPID"},
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage of %s:%s%s%s%s`, os.Args[0], commonFlag, serverFlag, clientFlag, svc.Usage())
	}
	flag.StringVar(&server.Host, "host", "", "Listening host")
	flag.StringVar(&server.Port, "port", "", "Listening port")
	flag.StringVar(&svc.Options.UpdateURL, "update", "", "Update URL")
	flags.SetConfigFile(filepath.Join(filepath.Dir(self), "config.ini"))
	flags.Parse()

	if *secrets == "" {
		if info, err := os.Stat(filepath.Join(filepath.Dir(self), "secrets")); err == nil && !info.IsDir() {
			*secrets = filepath.Join(filepath.Dir(self), "secrets")
		}
	}

	if *whitelist == "" {
		if info, err := os.Stat(filepath.Join(filepath.Dir(self), "whitelist")); err == nil && !info.IsDir() {
			*whitelist = filepath.Join(filepath.Dir(self), "whitelist")
		}
	}

	if *status == "" {
		*status = filepath.Join(filepath.Dir(self), "status")
	}

	if err := svc.ParseAndRun(flag.Args()); err != nil {
		svc.Fatal(err)
	}
}
