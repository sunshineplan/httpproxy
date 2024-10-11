package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
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

var (
	last            string
	customAutoproxy []byte
)

func fetchAutoproxy(c *Client) (string, error) {
	accessLogger.Print("fetch autoproxy")
	ch := make(chan []byte)
	defer close(ch)
	go func() {
		resp, err := http.Get(autoproxyURL)
		if err != nil {
			errorLogger.Debug("failed to check autoproxy without using proxy", "error", err)
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			errorLogger.Debug("failed to check autoproxy without using proxy", "error", err)
			return
		}
		select {
		case ch <- b:
		default:
		}
	}()
	go func() {
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			t = t.Clone()
			t.Proxy = http.ProxyURL(c.u)
			resp, err := (&http.Client{Transport: t}).Get(autoproxyURL)
			if err != nil {
				errorLogger.Debug("failed to check autoproxy using proxy", "error", err)
			}
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				errorLogger.Debug("failed to check autoproxy using proxy", "error", err)
				return
			}
			select {
			case ch <- b:
			default:
			}
		}
	}()
	var b []byte
	select {
	case <-time.After(time.Minute):
		return "", errors.New("failed to check autoproxy")
	case b = <-ch:
		accessLogger.Print("autoproxy fetched")
	}
	return string(b), nil
}

func addPerHost(p *proxy.PerHost, s string, custom bool) *proxy.PerHost {
	if custom {
		p.AddFromString(s)
	} else {
		for _, i := range txt.ReadAll(strings.NewReader(s)) {
			if strings.HasSuffix(i, "@cn") {
				continue
			}
			i = strings.ReplaceAll(i, ":@ads", "")
			switch {
			case strings.HasPrefix(i, "domain:"):
				p.AddZone(strings.TrimPrefix(i, "domain:"))
			case strings.HasPrefix(i, "full:"):
				p.AddHost(strings.TrimPrefix(i, "full:"))
			}
		}
	}
	return p
}

func parseAutoproxy(p *proxy.PerHost, s, custom string) *proxy.PerHost {
	addPerHost(p, s, false)
	addPerHost(p, custom, true)
	return p
}

func initAutoproxy(c *Client) *proxy.PerHost {
	accessLogger.Debug("autoproxy: " + *autoproxy)
	s, err := fetchAutoproxy(c)
	if err != nil {
		errorLogger.Print(err)
		return nil
	}
	accessLogger.Debug("custom autoproxy: " + *custom)
	customAutoproxy, err = os.ReadFile(*custom)
	if err != nil {
		errorLogger.Println("failed to read custom autoproxy file:", err)
	}
	p := parseAutoproxy(proxy.NewPerHost(
		&dialerLogger{"direct", proxy.Direct},
		&dialerLogger{"proxy", c.proxy},
	), s, string(customAutoproxy))
	scheduler.NewScheduler().At(scheduler.AtHour(12)).Do(func(_ time.Time) {
		s, err := fetchAutoproxy(c)
		if err != nil {
			errorLogger.Print(err)
			return
		}
		if s == last {
			accessLogger.Print("autoproxy: no update available")
			return
		}
		last = s
		c.autoproxy.Lock()
		defer c.autoproxy.Unlock()
		c.autoproxy.PerHost = parseAutoproxy(proxy.NewPerHost(
			&dialerLogger{"direct", proxy.Direct},
			&dialerLogger{"proxy", c.proxy},
		), s, string(customAutoproxy))
	})
	if err := watchFile(
		*custom,
		func() {
			c.autoproxy.Lock()
			defer c.autoproxy.Unlock()
			customAutoproxy, _ = os.ReadFile(*custom)
			c.autoproxy.PerHost = parseAutoproxy(proxy.NewPerHost(
				&dialerLogger{"direct", proxy.Direct},
				&dialerLogger{"proxy", c.proxy},
			), last, string(customAutoproxy))
		},
		func() {
			c.autoproxy.Lock()
			defer c.autoproxy.Unlock()
			customAutoproxy = nil
			c.autoproxy.PerHost = addPerHost(proxy.NewPerHost(
				&dialerLogger{"direct", proxy.Direct},
				&dialerLogger{"proxy", c.proxy},
			), last, false)
		},
	); err != nil {
		errorLogger.Print(err)
	}
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
	accessLogger.Printf("[%s] %s", d.name, hostname(address))
	return d.dialer.Dial(network, address)
}

func (d *dialerLogger) DialContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	accessLogger.Printf("[%s] %s", d.name, hostname(address))
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
