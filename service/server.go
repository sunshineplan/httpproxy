package main

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sunshineplan/limiter"
)

type Server struct {
	*Base
	tls     bool
	cert    string
	privkey string
}

func NewServer(base *Base) *Server {
	s := &Server{Base: base}
	s.Base.Handler = http.HandlerFunc(s.Handler)
	s.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	s.ReadTimeout = time.Minute * 10
	s.ReadHeaderTimeout = time.Second * 4
	s.WriteTimeout = time.Minute * 10
	return s
}

func (s *Server) SetTLS(cert, privkey string) *Server {
	s.tls = true
	s.cert = cert
	s.privkey = privkey
	return s
}

func (s *Server) Run() error {
	if s.tls {
		return s.RunTLS(s.cert, s.privkey)
	}
	return s.Base.Run()
}

func (*Server) HTTP(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
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

func (*Server) HTTPS(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
	dest_conn, err := net.DialTimeout("tcp", r.Host, 15*time.Second)
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

func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	user, lim, ok := s.Auth(w, r)
	if !ok {
		return
	}

	accessLogger.Printf("[S]%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		s.HTTPS(user, lim, w, r)
	} else {
		s.HTTP(user, lim, w, r)
	}
}
