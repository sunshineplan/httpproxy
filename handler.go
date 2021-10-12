package main

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	dest_conn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
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

	go transfer(dest_conn, client_conn)
	go transfer(client_conn, dest_conn)
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
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
	io.Copy(w, resp.Body)
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

func handler(w http.ResponseWriter, r *http.Request) {
	user := "anonymous"
	var pass string
	var ok bool
	if len(accounts) != 0 {
		user, pass, ok = parseBasicAuth(r.Header.Get("Proxy-Authorization"))
		if !ok {
			accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr)
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTPS Proxy"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		} else if !hasAccount(user, pass) {
			errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr)
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTPS Proxy"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
	}

	accessLogger.Printf("%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		handleTunneling(w, r)
	} else {
		handleHTTP(w, r)
	}
}
