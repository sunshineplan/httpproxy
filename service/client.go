package main

import (
	"bufio"
	"io"
	"net/http"
	"net/url"

	"github.com/sunshineplan/httpproxy"
	"github.com/sunshineplan/limiter"
	"golang.org/x/net/proxy"
)

type Client struct {
	*Base
	u     *url.URL
	proxy proxy.Dialer
}

func NewClient(base *Base, u *url.URL) *Client {
	c := &Client{Base: base, u: u, proxy: httpproxy.New(u, nil)}
	c.Base.Handler = http.HandlerFunc(c.Handler)
	return c
}

func (c *Client) SetProxyAuth(username, password string) *Client {
	if username != "" && password != "" {
		c.u.User = url.UserPassword(username, password)
	}
	return c
}

func (c *Client) HTTP(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
	port := r.URL.Port()
	if port == "" {
		port = "80"
	}
	conn, err := c.proxy.Dial("tcp", r.URL.Hostname()+":"+port)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := r.Write(conn); err != nil {
		conn.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), r)
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

func (c *Client) HTTPS(user string, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request) {
	dest_conn, err := c.proxy.Dial("tcp", r.Host)
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

	go transfer(dest_conn, client_conn, "", nil)
	go transfer(client_conn, dest_conn, user, lim)
}

func (c *Client) Handler(w http.ResponseWriter, r *http.Request) {
	user, lim, ok := c.Auth(w, r)
	if !ok {
		return
	}
	accessLogger.Printf("[C]%s[%s] %s %s", r.RemoteAddr, user, r.Method, r.URL)
	if r.Method == http.MethodConnect {
		c.HTTPS(user, lim, w, r)
	} else {
		c.HTTP(user, lim, w, r)
	}
}
