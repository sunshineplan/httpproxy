package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sunshineplan/utils/httpsvr"
	"github.com/sunshineplan/utils/retry"
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

func getAutoproxy(ctx context.Context, proxy *url.URL, c chan<- string) {
	mode := "default"
	client := http.DefaultClient
	if proxy == nil {
		mode = "no proxy"
		t, ok := http.DefaultTransport.(*http.Transport)
		if ok {
			t = t.Clone()
			t.Proxy = nil
			client = &http.Client{Transport: t}
		} else {
			client = &http.Client{Transport: &http.Transport{Proxy: nil}}
		}
	} else if proxy.String() != "" {
		mode = "proxy"
		t, ok := http.DefaultTransport.(*http.Transport)
		if ok {
			t = t.Clone()
			t.Proxy = http.ProxyURL(proxy)
			client = &http.Client{Transport: t}
		} else {
			client = &http.Client{Transport: &http.Transport{Proxy: nil}}
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", autoproxyURL, nil)
	if err != nil {
		errorLogger.Print(err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			errorLogger.Printf("failed to check autoproxy: %s: %s", mode, err)
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errorLogger.Println(mode, resp.StatusCode)
		return
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			errorLogger.Println(mode, err)
		}
		return
	}
	select {
	case c <- string(b):
	default:
	}
}

func fetchAutoproxy(c *Client) (string, error) {
	accessLogger.Print("fetch autoproxy")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ch := make(chan string)
	go getAutoproxy(ctx, nil, ch)
	go getAutoproxy(ctx, c.u, ch)
	go getAutoproxy(ctx, new(url.URL), ch)
	select {
	case <-ctx.Done():
		return "", errors.New("failed to check autoproxy")
	case b := <-ch:
		cancel()
		accessLogger.Print("autoproxy fetched")
		return string(b), nil
	}
}

func addPerHost(p *proxy.PerHost, s string, custom bool) *proxy.PerHost {
	if custom {
		p.AddFromString(s)
	} else {
		if res, err := txt.ReadAll(strings.NewReader(s)); err != nil {
			errorLogger.Print(err)
		} else {
			for _, i := range res {
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
	}
	return p
}

func parseAutoproxy(p *proxy.PerHost, s, custom string) *proxy.PerHost {
	addPerHost(p, s, false)
	addPerHost(p, custom, true)
	return p
}

func initAutoproxy(c *Client) *proxy.PerHost {
	var err error
	accessLogger.Debug("autoproxy: " + *autoproxy)
	if err = retry.Do(func() (err error) {
		last, err = fetchAutoproxy(c)
		return
	}, 3, 0); err != nil {
		errorLogger.Print(err)
	}
	accessLogger.Debug("custom autoproxy: " + *custom)
	customAutoproxy, err = os.ReadFile(*custom)
	if err != nil {
		errorLogger.Println("failed to load custom autoproxy file:", err)
	}
	p := parseAutoproxy(proxy.NewPerHost(
		&Dialer{UseDirect, proxy.Direct},
		&Dialer{UseProxy, c.proxy},
	), last, string(customAutoproxy))
	go func() {
		t := time.NewTicker(24 * time.Hour)
		for range t.C {
			s, err := fetchAutoproxy(c)
			if err != nil {
				errorLogger.Print(err)
				continue
			}
			if s == last {
				accessLogger.Print("autoproxy: no update available")
				continue
			}
			last = s
			c.autoproxy.Lock()
			c.autoproxy.PerHost = parseAutoproxy(proxy.NewPerHost(
				&Dialer{UseDirect, proxy.Direct},
				&Dialer{UseProxy, c.proxy},
			), s, string(customAutoproxy))
			c.autoproxy.Unlock()
		}
	}()
	if err := watchFile(
		*custom,
		func() {
			c.autoproxy.Lock()
			defer c.autoproxy.Unlock()
			customAutoproxy, _ = os.ReadFile(*custom)
			c.autoproxy.PerHost = parseAutoproxy(proxy.NewPerHost(
				&Dialer{UseDirect, proxy.Direct},
				&Dialer{UseProxy, c.proxy},
			), last, string(customAutoproxy))
		},
		func() {
			c.autoproxy.Lock()
			defer c.autoproxy.Unlock()
			customAutoproxy = nil
			c.autoproxy.PerHost = addPerHost(proxy.NewPerHost(
				&Dialer{UseDirect, proxy.Direct},
				&Dialer{UseProxy, c.proxy},
			), last, false)
		},
	); err != nil {
		errorLogger.Print(err)
	}
	return p
}
