package util

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

const httpClientTimeout = 5 * time.Minute

var httpClientPool *sync.Pool

func init() {
	httpClientPool = &sync.Pool{
		New: func() interface{} { return newBaseConfiguredHttpClient() },
	}
}

func newBaseConfiguredHttpClient() *http.Client {
	return &http.Client{
		Timeout:   httpClientTimeout,
		Transport: newConfiguredBaseTransport(),
	}
}

func newConfiguredBaseTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig:     &tls.Config{},
		Proxy:               http.ProxyFromEnvironment,
		DisableCompression:  false,
		DisableKeepAlives:   true,
		IdleConnTimeout:     20 * time.Second,
		MaxIdleConnsPerHost: 10,
		MaxIdleConns:        50,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 0,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}

}

func GetHTTPClient() *http.Client { return httpClientPool.Get().(*http.Client) }

func PutHTTPClient(c *http.Client) {
	c.Timeout = httpClientTimeout

	switch c.Transport.(type) {
	case *http.Transport:
		c.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = false
		httpClientPool.Put(c)
	default:
		c.Transport = newConfiguredBaseTransport()
		httpClientPool.Put(c)
	}
}
