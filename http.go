// Package httpproxy provides an HTTP proxy implementation
package httpproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// Dialer represents a proxy dialer
type Dialer struct {
	u                  *url.URL
	InsecureSkipVerify bool
	// ProxyDial specifies the optional dial function for
	// establishing the transport connection.
	ProxyDial func(context.Context, string, string) (net.Conn, error)
}

// New creates a new proxy Dialer
func New(u *url.URL, forward proxy.Dialer) proxy.Dialer {
	d := &Dialer{u: u}
	if forward != nil {
		if f, ok := forward.(proxy.ContextDialer); ok {
			d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
				return f.DialContext(ctx, network, address)
			}
		} else {
			d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
				return dialContext(ctx, forward, network, address)
			}
		}
	}
	return d
}

// connect establishes a connection to the proxy server
func (d *Dialer) connect(c net.Conn, network, address string) error {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return errors.New("network not implemented")
	}
	header := make(http.Header)
	if d.u.User != nil {
		password, _ := d.u.User.Password()
		header.Set("Proxy-Authorization", "Basic "+basicAuth(d.u.User.Username(), password))
	}
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: header,
	}
	if err := req.Write(c); err != nil {
		c.Close()
		return err
	}
	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		c.Close()
		return err
	}
	if resp.StatusCode != http.StatusOK {
		c.Close()
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return errors.New(resp.Status + " : " + string(b))
	}
	return nil
}

// Dial connects to the address on the named network using the proxy
func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("network not implemented")
	}
	if d.ProxyDial != nil {
		conn, err = d.ProxyDial(context.Background(), "tcp", d.u.Host)
	} else {
		conn, err = net.Dial("tcp", d.u.Host)
	}
	if err != nil {
		return
	}
	if d.u.Scheme == "https" {
		hostname, _, _ := strings.Cut(d.u.Host, ":")
		conn = tls.Client(conn, &tls.Config{ServerName: hostname, InsecureSkipVerify: d.InsecureSkipVerify})
	}
	if err = d.connect(conn, network, address); err != nil {
		conn.Close()
		return nil, err
	}
	return
}

// DialContext connects to the address on the named network using the proxy with the provided context
func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("network not implemented")
	}
	if d.ProxyDial != nil {
		conn, err = d.ProxyDial(ctx, "tcp", d.u.Host)
	} else {
		var dd net.Dialer
		conn, err = dd.DialContext(ctx, "tcp", d.u.Host)
	}
	if err != nil {
		return
	}
	if err = d.connect(conn, network, address); err != nil {
		conn.Close()
		return nil, err
	}
	return
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func dialContext(ctx context.Context, d proxy.Dialer, network, address string) (conn net.Conn, err error) {
	done := make(chan struct{}, 1)
	go func() {
		conn, err = d.Dial(network, address)
		close(done)
		if conn != nil && ctx.Err() != nil {
			conn.Close()
		}
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	return
}
