package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func initServer() func() error {
	server.Handler = http.HandlerFunc(serverHandler)
	server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	server.ReadTimeout = time.Minute * 10
	server.ReadHeaderTimeout = time.Second * 4
	server.WriteTimeout = time.Minute * 10
	if *https {
		return func() error {
			return server.RunTLS(*cert, *privkey)
		}
	} else {
		return server.Run
	}
}

func initClient() func() error {
	server.Handler = http.HandlerFunc(clientHandler)
	initProxy()
	return server.Run
}

func run() error {
	var run func() error
	switch mode := strings.ToLower(*mode); mode {
	case "server":
		if server.Port == "" {
			server.Port = "1080"
		}
		run = initServer()
	case "client":
		if server.Port == "" {
			server.Port = "8080"
		}
		run = initClient()
	default:
		return errors.New("unknow mode: " + mode)
	}
	initWhitelist()
	initSecrets()
	initDatabase()
	initStatus()
	defer func() {
		saveStatus()
		saveDatabase()
	}()
	return run()
}

func test() error {
	if server.Port == "" {
		switch strings.ToLower(*mode) {
		case "server":
			server.Port = "1080"
		case "client":
			server.Port = "8080"
		default:
			return errors.New("unknow mode:" + *mode)
		}
	}

	port, err := strconv.Atoi(server.Port)
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
