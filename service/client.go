package main

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/sunshineplan/httpproxy"
	"github.com/sunshineplan/httpproxy/auth"
	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/httpsvr"
	"golang.org/x/net/proxy"
)

type Client struct {
	*Base
	u     *url.URL
	proxy proxy.Dialer

	autoproxy *Autoproxy
}

func init() {
	proxy.RegisterDialerType("http", httpproxy.FromURL)
	proxy.RegisterDialerType("https", httpproxy.FromURL)
}

func NewClient(base *Base, u *url.URL) (*Client, error) {
	d, err := proxy.FromURL(u, nil)
	if err != nil {
		return nil, err
	}
	c := &Client{Base: base, u: u, proxy: d}
	c.Base.Handler = c.Handler(false)
	return c, nil
}

func (c *Client) SetProxyAuth(pa *proxy.Auth) *Client {
	if pa != nil {
		c.u.User = url.UserPassword(pa.User, pa.Password)
	} else {
		c.u.User = nil
	}
	if d, ok := c.proxy.(*httpproxy.Dialer); ok {
		if pa != nil {
			d.Auth = auth.Basic{Username: pa.User, Password: pa.Password}
		} else {
			d.Auth = nil
		}
	} else if c.u.Scheme == "socks5" || c.u.Scheme == "socks5h" {
		addr := c.u.Hostname()
		port := c.u.Port()
		if port == "" {
			port = "1080"
		}
		c.proxy, _ = proxy.SOCKS5("tcp", net.JoinHostPort(addr, port), pa, nil)
	}
	return c
}

func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	if d, ok := c.proxy.(*httpproxy.Dialer); ok {
		d.TLSConfig = config
	}
	return c
}

func (c *Client) SetAutoproxy(port string, autoproxy *proxy.PerHost) *Client {
	if port != "" && autoproxy != nil {
		server := httpsvr.New()
		server.Handler = c.Handler(true)
		server.Host = c.Base.Host
		server.Port = port
		c.autoproxy = &Autoproxy{Server: server, PerHost: autoproxy}
	}
	return c
}

func (c *Client) Run() error {
	if c.autoproxy != nil {
		go func() {
			if err := c.autoproxy.Run(); err != nil {
				c.Println("failed to run autoproxy:", err)
			}
		}()
	}
	return c.Base.Run()
}

func (c *Client) HTTP(user user, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request, autoproxy bool) {
	port := r.URL.Port()
	if port == "" {
		port = "80"
	}
	var conn net.Conn
	var err error
	if autoproxy {
		c.autoproxy.RLock()
		conn, err = c.autoproxy.Dial("tcp", net.JoinHostPort(r.URL.Hostname(), port))
		c.autoproxy.RUnlock()
	} else {
		conn, err = c.proxy.Dial("tcp", net.JoinHostPort(r.URL.Hostname(), port))
	}
	var name string
	if user.name != "" {
		name = "[" + user.name + "]"
	}
	var direct bool
	if t, ok := IsTyped(conn, err); ok {
		accessLogger.Printf("[%s]%s%s %s %s", t, r.RemoteAddr, name, r.Method, r.URL)
		if t == UseDirect {
			direct = true
		}
	} else {
		accessLogger.Printf("[C]%s%s %s %s", r.RemoteAddr, name, r.Method, r.URL)
	}
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
	if direct {
		io.Copy(w, resp.Body)
	} else {
		io.Copy(count(user, lim.Writer(w)), resp.Body)
	}
}

func (c *Client) HTTPS(u user, lim *limiter.Limiter, w http.ResponseWriter, r *http.Request, autoproxy bool) {
	var dest_conn net.Conn
	var err error
	if autoproxy {
		c.autoproxy.RLock()
		dest_conn, err = c.autoproxy.Dial("tcp", r.Host)
		c.autoproxy.RUnlock()
	} else {
		dest_conn, err = c.proxy.Dial("tcp", r.Host)
	}
	var name string
	if u.name != "" {
		name = "[" + u.name + "]"
	}
	var direct bool
	if t, ok := IsTyped(dest_conn, err); ok {
		accessLogger.Printf("[%s]%s%s %s %s", t, r.RemoteAddr, name, r.Method, r.URL)
		if t == UseDirect {
			direct = true
		}
	} else {
		accessLogger.Printf("[C]%s%s %s %s", r.RemoteAddr, name, r.Method, r.URL)
	}
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

	go transfer(dest_conn, client_conn, user{}, nil)
	if direct {
		go transfer(client_conn, dest_conn, user{}, nil)
	} else {
		go transfer(client_conn, dest_conn, u, lim)
	}
}

func (c *Client) Handler(autoproxy bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, lim, ok := c.Auth(w, r)
		if !ok {
			return
		}
		if r.Method == http.MethodConnect {
			c.HTTPS(user, lim, w, r, autoproxy)
		} else {
			c.HTTP(user, lim, w, r, autoproxy)
		}
	}
}
