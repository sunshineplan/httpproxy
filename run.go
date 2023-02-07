package main

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func runServer() {
	server.Handler = http.HandlerFunc(serverHandler)
	server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	server.ReadTimeout = time.Minute * 10
	server.ReadHeaderTimeout = time.Second * 4
	server.WriteTimeout = time.Minute * 10

	initLogger()
	initWhitelist()
	initSecrets()
	initStatus()

	var err error
	if *https {
		err = server.RunTLS(*cert, *privkey)
	} else {
		err = server.Run()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func runClient() {
	server.Handler = http.HandlerFunc(clientHandler)

	initProxy()
	initLogger()

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}

func run() {
	switch strings.ToLower(*mode) {
	case "server":
		if server.Port == "" {
			server.Port = "1080"
		}
		runServer()
	case "client":
		if server.Port == "" {
			server.Port = "8080"
		}
		runClient()
	default:
		log.Fatalln("unknow mode:", *mode)
	}
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
