package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"strconv"
)

func run() {
	server.Handler = http.HandlerFunc(handler)
	server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))

	initLogger()
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

func test() error {
	port, err := strconv.Atoi(server.Port)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}
	l.Close()
	return nil
}
