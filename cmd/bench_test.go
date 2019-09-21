package main

import (
	"net/http"
	"testing"

	"github.com/Smet1/golang-proxy/internal/pkg/httpclients"
)

func BenchmarkSimple(b *testing.B) {
	proxyClient := httpclients.ProxyHTTPClient("1.0.0.0:8888")
	req, err := http.NewRequest(http.MethodGet, "https://wikipedia.com", nil)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, err := proxyClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
