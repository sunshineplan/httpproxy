// Package httpproxy provides an HTTP proxy implementation
package httpproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/sunshineplan/httpproxy/auth"
	"golang.org/x/net/proxy"
)

// Dialer represents a proxy dialer
type Dialer struct {
	proxyAddress string

	// TLSConfig is the optional TLS configuration for HTTPS connections.
	TLSConfig *tls.Config

	// ProxyDial specifies the optional dial function for
	// establishing the transport connection.
	ProxyDial func(context.Context, string, string) (net.Conn, error)

	// Auth contains authentication information for the proxy.
	Auth auth.Authorization
}

// NewDialer returns a Dialer that makes HTTP connections to the given
// address with an optional username and password.
// If tlsConfig is provided, the Dialer will make HTTPS connecitons.
func NewDialer(address string, tlsConfig *tls.Config, pa *proxy.Auth, forward proxy.Dialer) (proxy.Dialer, error) {
	d := &Dialer{proxyAddress: address, TLSConfig: tlsConfig}
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
	if pa != nil {
		d.Auth = auth.Basic{
			Username: pa.User,
			Password: pa.Password,
		}
	}
	return d, nil
}

// FromURL returns a [proxy.Dialer] given a URL specification and an
// underlying Dialer for it to make network requests.
func FromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	var config *tls.Config
	switch u.Scheme {
	case "http":
	case "https":
		config = &tls.Config{ServerName: u.Hostname()}
	default:
		return nil, errors.New("httpproxy: unsupported scheme: " + u.Scheme)
	}
	port := u.Port()
	if port == "" {
		if config != nil {
			port = "443"
		} else {
			port = "80"
		}
	}
	var auth *proxy.Auth
	if u.User != nil {
		auth = new(proxy.Auth)
		auth.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			auth.Password = p
		}
	}
	return NewDialer(net.JoinHostPort(u.Hostname(), port), config, auth, forward)
}

// connect establishes a connection to the proxy server
func (d *Dialer) connect(c net.Conn, host string) error {
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: host},
		Host:   host,
		Header: make(http.Header),
	}
	if d.Auth != nil {
		d.Auth.Authorization(req)
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
		status := resp.Status
		if b, _ := io.ReadAll(resp.Body); len(b) > 0 {
			status += " : " + string(b)
		}
		return errors.New(status)
	}
	return nil
}

// Dial connects to the address on the named network using the proxy.
func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("network not implemented")
	}
	if d.ProxyDial != nil {
		conn, err = d.ProxyDial(context.Background(), "tcp", d.proxyAddress)
	} else {
		conn, err = net.Dial("tcp", d.proxyAddress)
	}
	if err != nil {
		return
	}
	if d.TLSConfig != nil {
		conn = tls.Client(conn, d.TLSConfig)
	}
	if err = d.connect(conn, address); err != nil {
		conn.Close()
		return nil, err
	}
	return
}

// DialContext connects to the address on the named network using the
// proxy with the provided context.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("network not implemented")
	}
	if d.ProxyDial != nil {
		conn, err = d.ProxyDial(ctx, "tcp", d.proxyAddress)
	} else {
		var dd net.Dialer
		conn, err = dd.DialContext(ctx, "tcp", d.proxyAddress)
	}
	if err != nil {
		return
	}
	if err = d.connect(conn, address); err != nil {
		conn.Close()
		return nil, err
	}
	return
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
