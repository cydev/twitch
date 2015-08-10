package main

import (
	"fmt"
	"github.com/cydev/twitch/api"
	"github.com/grafov/m3u8"
	"log"
	"net/http"
	"os"
	"time"
)

func getToSTDOUT(uri string) {
	res, err := http.Get(uri)
	if err != nil {
		log.Println("GET", uri, err)
	}
	defer res.Body.Close()
	p, _, err := m3u8.DecodeFrom(res.Body, true)
	if err != nil {
		log.Fatal(err)
	}
	switch p := p.(type) {
	case *m3u8.MediaPlaylist:
		{
			fmt.Println(p)
		}
	default:
		fmt.Println("wtf")
	}
}

var (
	targetQuality = "high"
)

func main() {
	tok, err := api.API.Token(api.TokenLive, os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	u := api.Usher.Channel(os.Args[1], tok.Values())
	res, err := http.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	p, _, err := m3u8.DecodeFrom(res.Body, true)
	if err != nil {
		log.Fatal(err)
	}
	for {
		switch p := p.(type) {
		case *m3u8.MasterPlaylist:
			{
				for _, variant := range p.Variants {
					if variant.Video != targetQuality {
						continue
					}
					fmt.Println(variant.Video)
					getToSTDOUT(variant.URI)
				}
			}
		default:
			fmt.Println("wtf")
		}
		time.Sleep(time.Second * 5)
	}
}
