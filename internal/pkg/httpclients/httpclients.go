package httpclients

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opencensus.io/plugin/ochttp"
)

func ProxyFuncCreater(proxyAddr string) func(*http.Request) (*url.URL, error) {
	var proxyFunc func(*http.Request) (*url.URL, error)
	proxyURL, proxyErr := url.Parse(proxyAddr)
	if proxyErr == nil && proxyAddr != "" {
		proxyFunc = http.ProxyURL(proxyURL)
	}
	return proxyFunc
}

func ProxyHTTPClient(proxyURL string) *http.Client {
	return &http.Client{
		Transport: &ochttp.Transport{Base: &http.Transport{
			Proxy: ProxyFuncCreater(proxyURL),
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		},
	}
}

func HTTPClient() *http.Client {
	return &http.Client{
		Transport: &ochttp.Transport{Base: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		},
	}
}
