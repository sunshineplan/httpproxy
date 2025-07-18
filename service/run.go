package main

import (
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"strconv"

	"github.com/sunshineplan/utils/httpsvr"
	"golang.org/x/net/proxy"
)

const (
	defaultServerPort = "8000"
	defaultClientPort = "8888"
)

type Runner interface {
	Run() error
}

func run() error {
	if addr := *pprof; addr != "" {
		go func() {
			svc.Print(http.ListenAndServe(addr, nil))
		}()
	}
	base := NewBase(*host, *port)
	base.ErrorLog = errorLogger.Logger
	servers := []*httpsvr.Server{base.Server}
	var runner Runner
	if *proxyAddr == "" {
		if base.Port == "" {
			base.Port = defaultServerPort
		}
		s := NewServer(base)
		if *https {
			s.SetTLS(*cert, *privkey)
		}
		runner = s
	} else {
		if base.Port == "" {
			base.Port = defaultClientPort
		}
		c, err := NewClient(base, parseProxy(*proxyAddr))
		if err != nil {
			return err
		}
		if *username != "" || *password != "" {
			c.SetProxyAuth(&proxy.Auth{User: *username, Password: *password})
		}
		if *autoproxy != "" {
			c.SetAutoproxy(*autoproxy, initAutoproxy(c))
			servers = append(servers, c.autoproxy.Server)
		}
		runner = c
	}
	base.accounts = initSecrets(*secrets)
	base.whitelist = initWhitelist(*whitelist)
	initRecord(base)
	initStatus(base, servers)
	defer func() {
		saveRecord(base)
		saveStatus(base, servers)
	}()
	return runner.Run()
}

func test() error {
	base := NewBase(*host, *port)
	if base.Port == "" {
		if *proxyAddr == "" {
			base.Port = defaultServerPort
		} else {
			base.Port = defaultClientPort
		}
	}

	port, err := strconv.Atoi(base.Port)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}
	l.Close()

	if *proxyAddr != "" {
		if _, err := url.Parse(*proxyAddr); err != nil {
			return err
		}
	}

	return nil
}
