package main

import (
	"bufio"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/sunshineplan/utils/httpproxy"
)

var p *httpproxy.Proxy

func initProxy() {
	proxyURL, err := url.Parse(*proxy)
	if err != nil {
		log.Fatalln("bad server address:", *proxy)
	}
	p = httpproxy.New(proxyURL, nil)
	if *debug {
		log.Print("Proxy ready")
	}
}

func clientTunneling(user string, w http.ResponseWriter, r *http.Request) {
	dest_conn, resp, err := p.DialWithHeader(r.Host, r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if resp.StatusCode != http.StatusOK {
		http.Error(w, resp.Status, resp.StatusCode)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	go transfer(dest_conn, client_conn, "")
	go transfer(client_conn, dest_conn, user)
}

func clientHTTP(user string, w http.ResponseWriter, r *http.Request) {
	port := r.URL.Port()
	if port == "" {
		port = "80"
	}
	conn, resp, err := p.DialWithHeader(r.Host+":"+port, r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		http.Error(w, resp.Status, resp.StatusCode)
		return
	}

	if err := r.Write(conn); err != nil {
		conn.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	br := bufio.NewReader(conn)
	resp, err = http.ReadResponse(br, r)
	if err != nil {
		conn.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	header := w.Header()
	for k, vv := range resp.Header {
		for _, v := range vv {
			header.Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)
	count(user, n)
}

func clientHandler(w http.ResponseWriter, r *http.Request) {
	if *username != "" && *password != "" {
		r.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(*username+":"+*password)))
	}
	accessLogger.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		clientTunneling(r.RemoteAddr, w, r)
	} else {
		clientHTTP(r.RemoteAddr, w, r)
	}
}
