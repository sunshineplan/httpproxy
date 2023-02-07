package main

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func transfer(dst io.WriteCloser, src io.ReadCloser, user string) {
	defer dst.Close()
	defer src.Close()
	n, _ := io.Copy(dst, src)
	if user != "" {
		count(user, uint64(n))
	}
}

func handleTunneling(user string, w http.ResponseWriter, r *http.Request) {
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

	go transfer(dest_conn, client_conn, "")
	go transfer(client_conn, dest_conn, user)
}

func handleHTTP(user string, w http.ResponseWriter, r *http.Request) {
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
	n, _ := io.Copy(w, resp.Body)
	count(user, uint64(n))
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
	if len(accounts) == 0 && len(allows) != 0 && !isAllow(r.RemoteAddr) {
		accessLogger.Printf("%s not allow", r.RemoteAddr)
		return
	} else if len(accounts) != 0 && !isAllow(r.RemoteAddr) {
		user, pass, ok = parseBasicAuth(r.Header.Get("Proxy-Authorization"))
		if !ok {
			accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr)
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return
		} else if !hasAccount(user, pass) {
			errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr)
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return
		}
		r.Header.Del("Proxy-Authorization")
	}

	accessLogger.Printf("%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		handleTunneling(user, w, r)
	} else {
		handleHTTP(user, w, r)
	}
}
