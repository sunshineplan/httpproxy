package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sunshineplan/service"
	"github.com/sunshineplan/utils/httpsvr"
	"github.com/vharitonsky/iniflags"
)

var (
	secrets   = flag.String("secrets", "", "Path to secrets file for Basic Authentication")
	https     = flag.Bool("https", false, "Serve as HTTPS proxy server")
	cert      = flag.String("cert", "", "Path to certificate file")
	privkey   = flag.String("privkey", "", "Path to private key file")
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
  --secrets <file>
    	Path to secrets file for Basic Authentication
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
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
	iniflags.SetConfigFile(filepath.Join(filepath.Dir(self), "config.ini"))
	iniflags.SetAllowMissingConfigFile(true)
	iniflags.SetAllowUnknownFlags(true)
	iniflags.Parse()

	if *secrets == "" {
		if info, err := os.Stat(filepath.Join(filepath.Dir(self), "secrets")); err == nil && !info.IsDir() {
			*secrets = filepath.Join(filepath.Dir(self), "secrets")
		}
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
		switch flag.Arg(0) {
		case "run":
			svc.Run(false)
		case "debug":
			svc.Run(true)
		case "test":
			err = svc.Test()
		case "install":
			err = svc.Install()
		case "uninstall", "remove":
			err = svc.Uninstall()
		case "start":
			err = svc.Start()
		case "stop":
			err = svc.Stop()
		case "restart":
			err = svc.Restart()
		case "update":
			err = svc.Update()
		default:
			log.Fatalln(fmt.Sprintf("Unknown argument: %s", flag.Arg(0)))
		}
	default:
		log.Fatalln(fmt.Sprintf("Unknown arguments: %s", strings.Join(flag.Args(), " ")))
	}
	if err != nil {
		log.Fatalf("Failed to %s: %v", flag.Arg(0), err)
	}
}
