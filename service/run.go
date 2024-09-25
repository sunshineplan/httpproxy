package main

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Runner interface {
	Run() error
}

func run() error {
	var runner Runner
	switch mode := strings.ToLower(*mode); mode {
	case "server":
		if base.Port == "" {
			base.Port = "1080"
		}
		s := NewServer(base)
		if *https {
			s.SetTLS(*cert, *privkey)
		}
		runner = s
	case "client":
		if base.Port == "" {
			base.Port = "8080"
		}
		runner = NewClient(base, parseProxy(*proxy)).SetProxyAuth(*username, *password)
	default:
		return errors.New("unknow mode: " + mode)
	}
	base.accounts = initSecrets(*secrets)
	base.whitelist = initWhitelist(*whitelist)
	initRecord(base)
	initStatus(base)
	defer func() {
		saveRecord(base)
		saveStatus(base)
	}()
	return runner.Run()
}

func test() error {
	if base.Port == "" {
		switch strings.ToLower(*mode) {
		case "server":
			base.Port = "1080"
		case "client":
			base.Port = "8080"
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
		if _, err := url.Parse(*proxy); err != nil {
			return err
		}
	}

	return nil
}
