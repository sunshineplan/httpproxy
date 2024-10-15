package main

import (
	"context"
	"net"

	"golang.org/x/net/proxy"
)

type DialerType int

const (
	UseDirect DialerType = iota + 1
	UseProxy
)

var typeList = map[DialerType]string{
	UseDirect: "direct",
	UseProxy:  "proxy",
}

func (t DialerType) String() string {
	return typeList[t]
}

type Dialer struct {
	DialerType
	proxy.Dialer
}

type Error struct {
	DialerType
	error
}

type Conn struct {
	net.Conn
	DialerType
}

type Typed interface {
	Type() DialerType
}

func NewConn(t DialerType) *Conn {
	return &Conn{DialerType: t}
}

func (c *Conn) WrapConn(conn net.Conn, err error) (net.Conn, error) {
	if err != nil {
		return nil, Error{c.DialerType, err}
	}
	c.Conn = conn
	return c, nil
}

func (c *Conn) Type() DialerType {
	return c.DialerType
}

func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return NewConn(d.DialerType).WrapConn(d.Dialer.Dial(network, address))
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	if f, ok := d.Dialer.(proxy.ContextDialer); ok {
		return NewConn(d.DialerType).WrapConn(f.DialContext(ctx, network, address))
	} else {
		return NewConn(d.DialerType).WrapConn(dialContext(ctx, d.Dialer, network, address))
	}
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

func IsTyped(conn net.Conn, err error) (DialerType, bool) {
	if conn != nil {
		if v, ok := conn.(Typed); ok {
			return v.Type(), true
		}
		return 0, false
	}
	if err != nil {
		if v, ok := conn.(Typed); ok {
			return v.Type(), true
		}
	}
	return 0, false
}
