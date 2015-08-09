package main

import (
	"os"
	"github.com/cydev/twitch/api"
	"log"
	"net/http"
	"io"
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
	defer res.Body.Close()
	io.Copy(os.Stdout, res.Body)
}
