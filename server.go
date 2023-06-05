package main

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sunshineplan/limiter"
)

func transfer(dst io.WriteCloser, src io.ReadCloser, user string, lim *limiter.Limiter) {
	defer dst.Close()
	defer src.Close()
	if lim == nil {
		io.Copy(count(user, dst), src)
	} else {
		io.Copy(count(user, lim.Writer(dst)), src)
	}
}

func serverTunneling(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
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
		dest_conn.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	go transfer(dest_conn, client_conn, "", nil)
	go transfer(client_conn, dest_conn, user, lim)
}

func serverHTTP(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
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
	io.Copy(count(user, lim.Writer(w)), resp.Body)
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

func serverHandler(w http.ResponseWriter, r *http.Request) {
	user, lim, ok := auth(w, r)
	if !ok {
		return
	}
	r.Header.Del("Proxy-Authorization")

	accessLogger.Printf("%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		serverTunneling(user, lim, w, r)
	} else {
		serverHTTP(user, lim, w, r)
	}
}
