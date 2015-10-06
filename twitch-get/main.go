package main

import (
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cydev/twitch/downloader"
	"log"
)

const (
	defaultHTTPTimeout        = 30 * time.Second
	defaultRequestTimeout     = 30 * time.Second
	defaultKeepAliveInterval  = 600 * time.Second
	defaultHTTPHeadersTimeout = defaultRequestTimeout
)

func getDefaultHTTPClient() *http.Client {
	client := &http.Client{
		Timeout: defaultRequestTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   defaultHTTPTimeout,
				KeepAlive: defaultKeepAliveInterval,
			}).Dial,
			TLSHandshakeTimeout:   defaultHTTPTimeout,
			ResponseHeaderTimeout: defaultHTTPHeadersTimeout,
		},
	}
	return client
}

func main() {
	client := getDefaultHTTPClient()
	streamName := os.Args[1]
	log.Println("recording streamer", streamName)
	d := downloader.New(streamName, client)
	d.Start()
}
