package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"maps"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sunshineplan/httpproxy"
	"github.com/sunshineplan/httpproxy/auth"
	"github.com/sunshineplan/limiter"
	"golang.org/x/net/proxy"
)

var testHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	m := make(map[string]string)
	for k, v := range r.Header {
		m[k] = r.Header.Get(k)
		header.Add(k, v[0])
	}
	b, _ := json.Marshal(m)
	w.Write(b)
})

func getPort(t *testing.T) string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
		return ""
	}
	defer listener.Close()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func newRequest(url string, m map[string]string) *http.Request {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "")
	for k, v := range m {
		req.Header.Set(k, v)
	}
	return req
}

func do(proxy proxy.Dialer, url string, req *http.Request) (resp *http.Response, m map[string]string, err error) {
	c, err := proxy.Dial("tcp", strings.TrimPrefix(url, "http://"))
	if err != nil {
		return
	}
	if err = req.WriteProxy(c); err != nil {
		return
	}
	resp, err = http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &m)
	return
}

func createCert() (string, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}
	certFile, err := os.CreateTemp("", "cert.pem")
	if err != nil {
		return "", "", err
	}
	defer certFile.Close()
	keyFile, err := os.CreateTemp("", "key.pem")
	if err != nil {
		return "", "", err
	}
	defer keyFile.Close()

	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certFile.Name(), keyFile.Name(), nil
}

func testProxy(t *testing.T, proxyPort, testURL string, m map[string]string) {
	req := newRequest(testURL, m)
	d, _ := httpproxy.NewDialer(":"+proxyPort, nil, nil, nil)
	resp, res, err := do(d, testURL, req)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(m, res) {
		t.Errorf("expect %v; got %v", m, res)
	}
	for k, v := range m {
		if vv := resp.Header.Get(k); vv != v {
			t.Errorf("expect %s %s; got %s", k, v, vv)
		}
	}
	u, err := url.Parse("http://localhost:" + proxyPort)
	if err != nil {
		t.Fatal(err)
	}
	d, _ = httpproxy.FromURL(u, nil)
	resp, res, err = do(d, testURL, req)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(m, res) {
		t.Errorf("expect %v; got %v", m, res)
	}
	for k, v := range m {
		if vv := resp.Header.Get(k); vv != v {
			t.Errorf("expect %s %s; got %s", k, v, vv)
		}
	}
}

func TestProxy(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()

	s := NewServer(NewBase("", getPort(t)))
	go s.Run()
	defer s.Shutdown(context.Background())

	c, _ := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
	go c.Run()
	defer c.Shutdown(context.Background())
	time.Sleep(time.Second)

	testProxy(t, s.Port, ts.URL, map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	})

	testProxy(t, c.Port, ts.URL, map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	})
}

func TestTLS(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()

	cert, privkey, err := createCert()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Remove(cert)
		os.Remove(privkey)
	}()

	s := NewServer(NewBase("", getPort(t))).SetTLS(cert, privkey)
	go s.Run()
	defer s.Shutdown(context.Background())

	c, _ := NewClient(NewBase("", getPort(t)), parseProxy("https://localhost:"+s.Port))
	c.SetTLSConfig(&tls.Config{ServerName: "localhost", InsecureSkipVerify: true})
	go c.Run()
	defer c.Shutdown(context.Background())
	time.Sleep(time.Second)

	testProxy(t, c.Port, ts.URL, map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	})
}

func TestAuth(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()
	serverUser := auth.Basic{Username: "server", Password: "server_password"}
	clientUser := auth.Basic{Username: "client", Password: "client_password"}
	m := map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	}
	req := newRequest(ts.URL, m)

	s := NewServer(NewBase("", getPort(t)))
	s.accounts.Store(serverUser, &limit{0, 0, limiter.New(limiter.Inf), nil})
	go s.Run()
	defer s.Shutdown(context.Background())

	for i, testcase := range []struct {
		client    func() *Client
		proxyAuth auth.Authorization
		err       string
	}{
		{
			func() *Client {
				c, _ := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
				return c
			},
			nil,
			"407 Proxy Authentication Required",
		},
		{
			func() *Client {
				c, _ := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
				return c.SetProxyAuth(&proxy.Auth{User: serverUser.Username, Password: serverUser.Password})
			},
			nil,
			"",
		},
		{
			func() *Client {
				c, _ := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
				c.SetProxyAuth(&proxy.Auth{User: serverUser.Username, Password: serverUser.Password})
				c.accounts.Store(clientUser, &limit{0, 0, limiter.New(limiter.Inf), nil})
				return c
			},
			nil,
			"407 Proxy Authentication Required",
		},
		{
			func() *Client {
				c, _ := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
				c.SetProxyAuth(&proxy.Auth{User: serverUser.Username, Password: serverUser.Password})
				c.accounts.Store(clientUser, &limit{0, 0, limiter.New(limiter.Inf), nil})
				return c
			},
			auth.Basic{Username: clientUser.Username, Password: clientUser.Password},
			"",
		},
	} {
		c := testcase.client()
		go c.Run()
		defer c.Shutdown(context.Background())
		time.Sleep(time.Second)

		d, _ := httpproxy.NewDialer(":"+c.Port, nil, nil, nil)
		if testcase.proxyAuth != nil {
			d.(*httpproxy.Dialer).Auth = testcase.proxyAuth
		}
		resp, res, err := do(d, ts.URL, req)
		if testcase.err != "" {
			if err == nil ||
				!strings.Contains(err.Error(), testcase.err) {
				t.Errorf("#%d expect %s, got %v", i, testcase.err, err)
			}
		} else {
			if err != nil {
				t.Error(i, err)
			} else {
				if !maps.Equal(m, res) {
					t.Errorf("%d expect %v; got %v", i, m, res)
				}
				for k, v := range m {
					if vv := resp.Header.Get(k); vv != v {
						t.Errorf("expect %s %s; got %s", k, v, vv)
					}
				}
			}
		}
	}
}
