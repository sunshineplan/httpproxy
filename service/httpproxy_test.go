package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	"github.com/sunshineplan/limiter"
	netproxy "golang.org/x/net/proxy"
)

var testHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	m := make(map[string]string)
	for k := range r.Header {
		m[k] = r.Header.Get(k)
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

func response(proxy netproxy.Dialer, addr string, req *http.Request) (m map[string]string, err error) {
	c, err := proxy.Dial("tcp", addr)
	if err != nil {
		return
	}
	if err = req.WriteProxy(c); err != nil {
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
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

func TestProxy(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")
	m := map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	}
	req := newRequest(ts.URL, m)

	s := NewServer(NewBase("", getPort(t)))
	go s.Run()
	defer s.Shutdown(context.Background())

	c := NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
	go c.Run()
	defer c.Shutdown(context.Background())
	time.Sleep(time.Second)

	u, _ := url.Parse("http://localhost:" + c.Port)
	res, err := response(httpproxy.New(u, nil), addr, req)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(m, res) {
		t.Errorf("expect %v; got %v", m, res)
	}
}

func TestTLS(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")
	m := map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	}
	req := newRequest(ts.URL, m)
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

	c := NewClient(NewBase("", getPort(t)), parseProxy("https://localhost:"+s.Port))
	c.proxy.(*httpproxy.Dialer).InsecureSkipVerify = true
	go c.Run()
	defer c.Shutdown(context.Background())
	time.Sleep(time.Second)

	u, _ := url.Parse("http://localhost:" + c.Port)
	res, err := response(httpproxy.New(u, nil), addr, req)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(m, res) {
		t.Errorf("expect %v; got %v", m, res)
	}
}

func TestAuth(t *testing.T) {
	ts := httptest.NewServer(testHandler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")
	user := account{"server", "password"}
	m := map[string]string{
		"Hello":               "world",
		"Proxy-Authorization": "Hello world!",
	}
	req := newRequest(ts.URL, m)

	s := NewServer(NewBase("", getPort(t)))
	s.accounts.Store(user, &limit{0, 0, limiter.New(limiter.Inf), nil})
	go s.Run()
	defer s.Shutdown(context.Background())

	for i, testcase := range []struct {
		client func() *Client
		proxy  string
		err    string
	}{
		{
			func() *Client {
				return NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port))
			},
			"http://localhost",
			"407 Proxy Authentication Required",
		},
		{
			func() *Client {
				return NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port)).
					SetProxyAuth(user.name, user.password)
			},
			"http://localhost",
			"",
		},
		{
			func() (c *Client) {
				c = NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port)).
					SetProxyAuth(user.name, user.password)
				c.accounts.Store(account{"client", "pwd"}, &limit{0, 0, limiter.New(limiter.Inf), nil})
				return
			},
			"http://localhost",
			"407 Proxy Authentication Required",
		},
		{
			func() (c *Client) {
				c = NewClient(NewBase("", getPort(t)), parseProxy("http://localhost:"+s.Port)).
					SetProxyAuth(user.name, user.password)
				c.accounts.Store(account{"client", "pwd"}, &limit{0, 0, limiter.New(limiter.Inf), nil})
				return
			},
			"http://client:pwd@localhost",
			"",
		},
	} {
		c := testcase.client()
		go c.Run()
		defer c.Shutdown(context.Background())
		time.Sleep(time.Second)

		u, _ := url.Parse(testcase.proxy + ":" + c.Port)
		res, err := response(httpproxy.New(u, nil), addr, req)
		if testcase.err != "" {
			if err == nil ||
				!strings.Contains(err.Error(), testcase.err) {
				t.Errorf("#%d expect %s, got %v", i, testcase.err, err)
			}
		} else {
			if err != nil {
				t.Error(err)
			} else if !maps.Equal(m, res) {
				t.Errorf("%d expect %v; got %v", i, m, res)
			}
		}
	}
}
