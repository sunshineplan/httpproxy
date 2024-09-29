package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sunshineplan/utils/httpsvr"
	"github.com/sunshineplan/utils/scheduler"
	"github.com/sunshineplan/utils/txt"
	"golang.org/x/net/proxy"
)

type Autoproxy struct {
	sync.RWMutex
	*httpsvr.Server
	*proxy.PerHost
}

const autoproxyURL = "https://raw.githubusercontent.com/v2fly/domain-list-community/release/geolocation-!cn.txt"

var last string

var errNoUpdateAvailable = errors.New("no update available")

func parseAutoproxy(c *Client) (*proxy.PerHost, error) {
	resp, err := http.Get(autoproxyURL)
	if err != nil {
		if t, ok := http.DefaultClient.Transport.(*http.Transport); ok {
			t.Proxy = http.ProxyURL(c.u)
			if resp, err = http.Get(autoproxyURL); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if string(b) == last {
		return nil, errNoUpdateAvailable
	}
	perHost := proxy.NewPerHost(
		&dialerLogger{"direct", proxy.Direct},
		&dialerLogger{"proxy", c.proxy},
	)
	for _, i := range txt.ReadAll(bytes.NewReader(b)) {
		if strings.HasSuffix(i, "@cn") {
			continue
		}
		i = strings.ReplaceAll(i, ":@ads", "")
		switch {
		case strings.HasPrefix(i, "domain:"):
			perHost.AddZone(strings.TrimPrefix(i, "domain:"))
		case strings.HasPrefix(i, "full:"):
			perHost.AddHost(strings.TrimPrefix(i, "full:"))
		}
	}
	return perHost, nil
}

func initAutoproxy(c *Client) *proxy.PerHost {
	accessLogger.Debug("autoproxy: " + *autoproxy)
	p, err := parseAutoproxy(c)
	if err != nil {
		errorLogger.Print(err)
		return nil
	}
	scheduler.NewScheduler().At(scheduler.AtHour(0)).Do(func(_ time.Time) {
		c.autoproxy.Lock()
		defer c.autoproxy.Unlock()
		p, err := parseAutoproxy(c)
		if err != nil {
			if err != errNoUpdateAvailable {
				errorLogger.Print(err)
			}
			return
		}
		accessLogger.Print("autoproxy updated")
		c.autoproxy.PerHost = p
	})
	return p
}

type dialerLogger struct {
	name   string
	dialer proxy.Dialer
}

func hostname(address string) string {
	host, _, _ := net.SplitHostPort(address)
	return host
}

func (d *dialerLogger) Dial(network, address string) (net.Conn, error) {
	accessLogger.Printf("[A] %s %s", d.name, hostname(address))
	return d.dialer.Dial(network, address)
}

func (d *dialerLogger) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	accessLogger.Printf("[A] %s %s", d.name, hostname(address))
	if f, ok := d.dialer.(proxy.ContextDialer); ok {
		return f.DialContext(ctx, network, address)
	} else {
		return dialContext(ctx, d.dialer, network, address)
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
