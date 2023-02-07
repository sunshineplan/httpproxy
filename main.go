package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sunshineplan/service"
	"github.com/sunshineplan/utils/flags"
	"github.com/sunshineplan/utils/httpsvr"
)

var (
	secrets   = flag.String("secrets", "", "Path to secrets file for Basic Authentication")
	https     = flag.Bool("https", false, "Serve as HTTPS proxy server")
	cert      = flag.String("cert", "", "Path to certificate file")
	privkey   = flag.String("privkey", "", "Path to private key file")
	whitelist = flag.String("whitelist", "", "Path to whitelist file")
	status    = flag.String("status", "", "Path to status file")
	keep      = flag.Int("keep", 100, "Count of status files")
	accesslog = flag.String("access-log", "", "Path to access log file")
	errorlog  = flag.String("error-log", "", "Path to error log file")
	debug     = flag.Bool("debug", false, "debug")
)

var self string
var server = httpsvr.New()

var svc = service.Service{
	Name:     "HTTPProxy",
	Desc:     "HTTP(S) Proxy Server",
	Exec:     run,
	TestExec: test,
	Options: service.Options{
		Dependencies: []string{"After=network.target"},
		Others:       []string{"ExecReload=kill -HUP $MAINPID"},
	},
}

func init() {
	var err error
	self, err = os.Executable()
	if err != nil {
		log.Fatalln("Failed to get self path:", err)
	}
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), `Usage of %s:
  --host <string>
    	Listening host
  --port <number>
    	Listening port
  --whitelist <file>
    	Path to whitelist file
  --secrets <file>
    	Path to secrets file for Basic Authentication
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
  --status <file>
    	Path to status file
  --access-log <file>
    	Path to access log file
  --error-log <file>
    	Path to error log file
  --update <url>
    	Update URL
%s`, os.Args[0], service.Usage)
}

func main() {
	flag.Usage = usage
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

	if *status == "" {
		*status = filepath.Join(filepath.Dir(self), "status")
	}

	if service.IsWindowsService() {
		svc.Run(false)
		return
	}

	var err error
	switch flag.NArg() {
	case 0:
		run()
	case 1:
		cmd := flag.Arg(0)
		var ok bool
		if ok, err = svc.Command(cmd); !ok {
			log.Fatalln("Unknown argument:", cmd)
		}
	default:
		log.Fatalln("Unknown arguments:", strings.Join(flag.Args(), " "))
	}
	if err != nil {
		log.Fatalf("Failed to %s: %v", flag.Arg(0), err)
	}
}
