package main

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

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
	base := NewBase(*host, *port)
	base.ErrorLog = errorLogger.Logger
	var runner Runner
	switch mode := strings.ToLower(*mode); mode {
	case "server":
		if base.Port == "" {
			base.Port = defaultServerPort
		}
		s := NewServer(base)
		if *https {
			s.SetTLS(*cert, *privkey)
		}
		runner = s
	case "client":
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
		}
		runner = c
	default:
		return errors.New("unknow mode: " + mode)
	}
	base.accounts = initSecrets(*secrets)
	base.whitelist = initWhitelist(*whitelist)
	initRecord(base)
	initStatus(base, []*httpsvr.Server{base.Server})
	defer func() {
		saveRecord(base)
		saveStatus(base, []*httpsvr.Server{base.Server})
	}()
	return runner.Run()
}

func test() error {
	base := NewBase(*host, *port)
	if base.Port == "" {
		switch strings.ToLower(*mode) {
		case "server":
			base.Port = defaultServerPort
		case "client":
			base.Port = defaultClientPort
		default:
			return errors.New("unknow mode:" + *mode)
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

	if strings.ToLower(*mode) == "client" {
		if _, err := url.Parse(*proxyAddr); err != nil {
			return err
		}
	}

	return nil
}
