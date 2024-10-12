package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sunshineplan/service"
	"github.com/sunshineplan/utils/flags"
)

// common flags
var (
	host      = flag.String("host", "", "Listening host")
	port      = flag.String("port", "", "Listening port")
	mode      = flag.String("mode", "server", "server or client mode")
	accesslog = flag.String("access-log", "", "Path to access log file")
	errorlog  = flag.String("error-log", "", "Path to error log file")
	secrets   = flag.String("secrets", "", "Path to secrets file for Basic Authentication")
	whitelist = flag.String("whitelist", "", "Path to whitelist file")
	status    = flag.String("status", "", "Path to status file")
	keep      = flag.Int("keep", 100, "Count of status files")
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
  --secrets <file>
    	Path to secrets file for Basic Authentication
  --whitelist <file>
    	Path to whitelist file
  --status <file>
    	Path to status file
  --keep number
    	Count of status files (default: 100)
  --update <url>
    	Update URL
`

// server flags
var (
	https   = flag.Bool("https", false, "Serve as HTTPS proxy server")
	cert    = flag.String("cert", "", "Path to certificate file")
	privkey = flag.String("privkey", "", "Path to private key file")
)

const serverFlag = `
server side:
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
`

// client flags
var (
	proxyAddr = flag.String("proxy", "", "Proxy address")
	username  = flag.String("username", "", "Username")
	password  = flag.String("password", "", "Password")
	autoproxy = flag.String("autoproxy", "", "Auto proxy listening port")
	custom    = flag.String("custom", "", "Path to custom autoproxy file")
)

const clientFlag = `
client side:
  --proxy <string>
    	Proxy address
  --username <string>
    	Username for Basic Authentication
  --password <string>
    	Password for Basic Authentication
  --autoproxy <string>
    	Auto proxy listening port
`

var svc = service.New()

func init() {
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
	self, err := os.Executable()
	if err != nil {
		log.Fatalln("Failed to get self path:", err)
	}
	recordFile = filepath.Join(filepath.Dir(self), "database")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage of %s:%s%s%s%s`, os.Args[0], commonFlag, serverFlag, clientFlag, svc.Usage())
	}
	flag.StringVar(&svc.Options.UpdateURL, "update", "", "Update URL")
	flags.SetConfigFile(filepath.Join(filepath.Dir(self), "config.ini"))
	flags.Parse()

	if *secrets == "" {
		*secrets = filepath.Join(filepath.Dir(self), "secrets")
	}
	if *whitelist == "" {
		*whitelist = filepath.Join(filepath.Dir(self), "whitelist")
	}
	if *status == "" {
		*status = filepath.Join(filepath.Dir(self), "status")
	}
	if *custom == "" {
		*custom = filepath.Join(filepath.Dir(self), "autoproxy.txt")
	}

	initLogger()

	if err := svc.ParseAndRun(flag.Args()); err != nil {
		svc.Fatal(err)
	}
}
