package main

import (
	"bufio"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/httpproxy"
)

var p *httpproxy.Proxy

func initProxy() {
	if *debug {
		accessLogger.Println("proxy:", *proxy)
	}
	proxyURL, err := url.Parse(*proxy)
	if err != nil {
		log.Fatalln("bad server address:", *proxy)
	}
	p = httpproxy.New(proxyURL, nil)
	if *debug {
		accessLogger.Print("Proxy ready")
	}
}

func clientTunneling(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
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

	go transfer(dest_conn, client_conn, "", nil)
	go transfer(client_conn, dest_conn, user, lim)
}

func clientHTTP(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
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
	io.Copy(count(user, lim.Writer(w)), resp.Body)
}

func clientHandler(w http.ResponseWriter, r *http.Request) {
	user, lim, ok := auth(w, r)
	if !ok {
		return
	}
	if *username != "" && *password != "" {
		r.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(*username+":"+*password)))
	} else {
		r.Header.Del("Proxy-Authorization")
	}
	accessLogger.Printf("%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		clientTunneling(user, lim, w, r)
	} else {
		clientHTTP(user, lim, w, r)
	}
}
