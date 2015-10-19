package api

import (
	"testing"

	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	. "github.com/smartystreets/goconvey/convey"
)

func TestToken(t *testing.T) {
	Convey("Token", t, func() {
		tok := Token{}
		tok.Sig = "test"
		tok.Body = `{"a": "b", "c": 1234}`
		v := tok.Values()
		So(v.Get("sig"), ShouldEqual, "test")
		So(v.Get("token"), ShouldEqual, tok.Body)
	})
}

type MockHTTPClient struct {
	response *http.Response
	request  *http.Request
	err      error
	callback func(req *http.Request) (*http.Response, error)
}

func (m MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.request = req
	if m.callback != nil {
		return m.callback(req)
	}
	return m.response, m.err
}

func jsonResponse(body string, code int) (res *http.Response) {
	res = &http.Response{}
	res.StatusCode = code
	res.Body = ioutil.NopCloser(bytes.NewBufferString(body))
	return res
}

func TestAPI(t *testing.T) {
	Convey("API", t, func() {
		Convey("OK", func() {
			m := &MockHTTPClient{}
			m.response = jsonResponse(`{"token": "{\"a\": \"b\"}", "sig": "abcdef"}`, http.StatusOK)
			client := TwitchAPI{httpClient: m}
			t, err := client.Token(TokenLive, "test")
			So(err, ShouldBeNil)
			So(t.Body, ShouldNotBeBlank)
		})
		Convey("Err", func() {
			m := &MockHTTPClient{}
			m.err = io.ErrUnexpectedEOF
			m.response = jsonResponse(`{"token": "{\"a\": \"b\"}", "sig": "abcdef"}`, http.StatusOK)
			client := TwitchAPI{httpClient: m}
			_, err := client.Token(TokenLive, "test")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestUsher(t *testing.T) {
	Convey("Usher", t, func() {
		Convey("URL", func() {
			values := url.Values{}
			values.Add("foo", "bar")
			url := Usher.URL("endpoint", values)
			So(url.Query().Get("foo"), ShouldEqual, "bar")
			So(url.Path, ShouldEqual, "endpoint")
			Convey("Channel", func() {
				So(Usher.Channel("cauthontv", nil).Path, ShouldEqual, "api/channel/hls/cauthontv.m3u8")
			})
			Convey("Video", func() {
				So(Usher.Video("12345", nil).Path, ShouldEqual, "vod/12345")
			})
		})
	})
}
