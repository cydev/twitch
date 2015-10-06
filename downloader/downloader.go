package downloader

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cydev/twitch/api"
	"github.com/golang/groupcache/lru"
	"github.com/grafov/m3u8"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

const (
	targetVideo     = "chunked"
	checkInterval   = time.Second * 3
	downloadInterval = time.Second * 8
	maxCacheEntries = 128
)

var (
	ErrStreamOffline       = errors.New("Stream offline")
	ErrTargetVideoNotFound = errors.New("Target not found")
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Chunk struct {
	URL  string
	Name string
	ID   int64
}

type Stream struct {
	Name string
	URL  string
}

type Downloader struct {
	cache      *lru.Cache
	httpClient HTTPClient
	dir        string
	channel    string
	out        *os.File
}

func getFileName(stream string, time time.Time) string {
	return fmt.Sprintf("%s-%s.mp4", stream, time.Format("02-01-06"))
}

func (d Downloader) getStream() (stream Stream, err error) {
	tok, err := api.API.Token(api.TokenLive, d.channel)
	if err != nil {
		return
	}
	u := api.Usher.Channel(d.channel, tok.Values())
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return
	}
	res, err := d.httpClient.Do(req)
	if err != nil {
		return
	}
	p, _, err := m3u8.DecodeFrom(res.Body, true)
	if err != nil {
		return stream, ErrStreamOffline
	}
	switch p := p.(type) {
	case *m3u8.MasterPlaylist:
		{
			for _, variant := range p.Variants {
				if variant.Video != targetVideo {
					continue
				}
				stream.URL = variant.URI
				return stream, nil
			}
		}
	}
	return stream, ErrTargetVideoNotFound
}

func (d Downloader) DownloadChunk(chunkURL string) error {
	log.Println("GET", chunkURL)
	req, err := http.NewRequest("GET", chunkURL, nil)
	if err != nil {
		log.Println("new_request err", err)
		return err
	}
	res, err := d.httpClient.Do(req)
	if err != nil {
		log.Println("HTTP ERR", err)
		return err
	}
	defer res.Body.Close()
	if _, err = io.Copy(d.out, res.Body); err != nil {
		log.Println("IO ERR", err)
		return err
	}
	return nil
}

func (d *Downloader) prepareFile() error {
	fileName := getFileName(d.channel, time.Now())
	filePath := filepath.Join(d.dir, fileName)
	log.Println("filepath:", filePath)
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		f, err := os.Create(filePath)
		if err != nil {
			return err
		}
		d.out = f
		return nil
	}
	if err != nil {
		log.Println("stat err", err)
		return err
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	d.out = f
	return nil
}

func (d Downloader) DownloadChunks(stream Stream) error {
	log.Println("downloading chunks")
	req, err := http.NewRequest("GET", stream.URL, nil)
	if err != nil {
		return err
	}
	res, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	p, _, err := m3u8.DecodeFrom(res.Body, true)
	if err != nil {
		return err
	}
	playlistURL, err := url.Parse(stream.URL)
	if err != nil {
		return err
	}
	var (
		chunkURL string
	)
	switch p := p.(type) {
	case *m3u8.MediaPlaylist:
		{
			for _, segment := range p.Segments {
				if segment == nil {
					continue
				}
				chunkURL = segment.URI
				if !strings.HasPrefix(chunkURL, "http") {
					u, err := playlistURL.Parse(segment.URI)
					if err != nil {
						log.Println("parse", err)
						continue
					}
					chunkURL = u.String()
				}
				_, hit := d.cache.Get(chunkURL)
				if hit {
					continue
				}
				if err := d.DownloadChunk(chunkURL); err != nil {
					log.Println("chunk download error", err)
				} else {
					d.cache.Add(chunkURL, nil)
				}
			}
		}
	default:
		return errors.New("Bad playlist type")
	}
	return nil
}

func (d *Downloader) Download(stream Stream) error {
	log.Println("start of record")
	defer log.Println("end of record")
	if err := d.prepareFile(); err != nil {
		return err
	}
	defer d.out.Close()
	ticker := time.NewTicker(downloadInterval)
	for _ = range ticker.C {
		if err := d.DownloadChunks(stream); err != nil {
			return err
		}
	}
	return nil
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(checkInterval)
	for _ = range ticker.C {
		stream, err := d.getStream()
		if err == ErrStreamOffline {
			continue
		}
		if err != nil {
			log.Println("error", err)
		}
		if err := d.Download(stream); err != nil {
			log.Println("download error", err)
		}
	}
}

func (d *Downloader) Start() {
	d.loop()
}

func New(name string, client HTTPClient) *Downloader {
	d := new(Downloader)
	d.channel = name
	d.cache = lru.New(maxCacheEntries)
	d.httpClient = client

	return d
}
