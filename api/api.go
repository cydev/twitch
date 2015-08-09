package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type UsherAPI struct{}

type TwitchAPI struct {
	httpClient HTTPClient
}

type TokenType string

const (
	TokenVideo TokenType = "vods"
	TokenLive  TokenType = "channels"
)

type Token struct {
	Body             string `json:"token"`
	Sig              string `json:"sig"`
	MobileRestricted bool   `json:"mobile_restricted"`
}

func (tok Token) Values() (u url.Values) {
	u = url.Values{}
	u.Add("sig", tok.Sig)
	u.Add("token", tok.Body)
	return u
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func mustReq(method string, url *url.URL, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url.String(), body)
	must(err)
	return req
}

func mustGet(u *url.URL) *http.Request {
	return mustReq("GET", u, nil)
}

func (api TwitchAPI) Token(t TokenType, value string) (token Token, err error) {
	var u *url.URL
	if t == TokenLive {
		u = api.TokenURL("channels", value, nil)
	} else {
		u = api.TokenURL("vods", value, nil)
	}
	res, err := api.httpClient.Do(mustGet(u))
	if err != nil {
		return token, err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&token); err != nil {
		return token, err
	}
	return token, nil
}

func (api TwitchAPI) URL(endpoint string, params url.Values) (u *url.URL) {
	u = new(url.URL)
	u.Path = fmt.Sprintf("%s.json", endpoint)
	u.Host = "api.twitch.tv"
	u.Scheme = "https"
	values := u.Query()
	for k, v := range params {
		values[k] = v
	}
	u.RawQuery = values.Encode()
	return u
}

func (api TwitchAPI) TokenURL(endpoint, asset string, params url.Values) (u *url.URL) {
	return api.URL(path.Join("api", endpoint, asset, "access_token"), params)
}

func randInt(max int) int {
	return rand.Intn(max)
}

func (api UsherAPI) URL(endpoint string, params url.Values) (u *url.URL) {
	u = new(url.URL)
	u.Path = endpoint
	u.Host = "usher.twitch.tv"
	u.Scheme = "http"
	values := u.Query()
	values.Add("player", "twitchweb")
	values.Add("p", strconv.Itoa(randInt(999999)))
	values.Add("type", "any")
	values.Add("allow_source", "true")
	values.Add("allow_audio_only", "true")
	for k, v := range params {
		values[k] = v
	}
	u.RawQuery = values.Encode()
	return u
}

func (api UsherAPI) Channel(name string, params url.Values) (u *url.URL) {
	return api.URL(path.Join("api", "channel", "hls", fmt.Sprintf("%s.m3u8", name)), params)
}

func (api UsherAPI) Video(video_id string, params url.Values) (u *url.URL) {
	return api.URL(path.Join("vod", video_id), params)
}

var (
	Usher = UsherAPI{}
	API   = TwitchAPI{http.DefaultClient}
)
