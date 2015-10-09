package downloader

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cydev/twitch/api"
	"github.com/cydev/twitch/telegram"
	"github.com/golang/groupcache/lru"
	"github.com/grafov/m3u8"
)

const (
	targetVideo      = "chunked"
	checkInterval    = time.Second * 3
	downloadInterval = time.Second * 8
	maxCacheEntries  = 128

	metadataExtension = "info"
)

var (
	ErrStreamOffline       = errors.New("Stream offline")
	ErrTargetVideoNotFound = errors.New("Target not found")
	workdir                string
	chatRoom int
	telegramToken string
)

func init() {
	flag.StringVar(&workdir, "dir", "", "Working directory")
	flag.IntVar(&chatRoom, "chat", 1863832, "Telegram chat id")
	flag.StringVar(&telegramToken, "telegram-token", "", "Token for telegram bot")
}

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
	fileName   string
	active     bool
	notifier   telegram.Notifier
}

type Metadata struct {
	Title  string
	Author string
	Date   time.Time
}

func ReadMetadata(input io.Reader) (metadata Metadata, err error) {
	decoder := json.NewDecoder(input)
	return metadata, decoder.Decode(&metadata)
}

func WriteMetadata(output io.Writer, metadata Metadata) (err error) {
	encoder := json.NewEncoder(output)
	return encoder.Encode(metadata)
}

func GetMetadataFileName(fileName string) string {
	return fmt.Sprintf("%s.%s", fileName, metadataExtension)
}

func getFileName(stream string, time time.Time) string {
	return fmt.Sprintf("%s-%s.mp4", stream, time.Format("02-01-06"))
}

func (d Downloader) Notify(message string) {
	if err := d.notifier.Notify(message); err != nil {
		log.Println("notification failed:", err)
	}
}

func (d Downloader) getMetadata() (metadata Metadata, err error) {
	c, err := api.API.Channel(d.channel)
	if err != nil {
		return metadata, err
	}
	if c.Stream == nil {
		return metadata, ErrStreamOffline
	}
	metadata.Date = c.Stream.CreatedAt
	metadata.Author = c.Stream.Data.Name
	metadata.Title = c.Stream.Data.Status

	return metadata, nil
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
	d.fileName = fileName
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
					d.notify("chunk download error", err)
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
	d.Notify(fmt.Sprintf("Начата запись для канала %s", d.channel))
	defer log.Println("end of record")
	defer d.Notify(fmt.Sprintf("Запись для канала %s завершена", d.channel))
	if err := d.prepareFile(); err != nil {
		return err
	}
	defer d.out.Close()
	ticker := time.NewTicker(downloadInterval)
	d.active = true
	defer func() {
		d.active = false
	}()
	for _ = range ticker.C {
		if err := d.DownloadChunks(stream); err != nil {
			return err
		}
	}
	return nil
}

func (d *Downloader) writeMetadata(metadata Metadata) (err error) {
	metadataPath := path.Join(d.dir, GetMetadataFileName(d.fileName))
	log.Println("writing metadata to file:", metadataPath)
	f, err := os.Create(metadataPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteMetadata(f, metadata)
}

func (d *Downloader) metadataLoop() {
	ticker := time.NewTicker(checkInterval)
	var (
		metadataSaved = false
	)
	for _ = range ticker.C {
		if !d.active {
			metadataSaved = false
			continue
		}
		if metadataSaved {
			continue
		}
		metadata, err := d.getMetadata()
		if err == ErrStreamOffline {
			continue
		}
		if err != nil {
			log.Println("metatada get failed:", err)
		}
		if err := d.writeMetadata(metadata); err != nil {
			log.Println("metadata write failed:", err)
		} else {
			metadataSaved = true
		}
	}
}

func (d *Downloader) notify(v ...interface{}) {
	s := fmt.Sprintln(v...)
	log.Print(s)
	d.Notify(s)
}

func (d *Downloader) loop() {
	ticker := time.NewTicker(checkInterval)
	for _ = range ticker.C {
		stream, err := d.getStream()
		if err == ErrStreamOffline {
			continue
		}
		if err != nil {
			d.notify("error", err)
			continue
		}
		if err := d.Download(stream); err != nil {
			d.notify("download error", err)
		}
	}
}

func (d *Downloader) Start() {
	go d.metadataLoop()
	d.loop()
}

func New(name string, client HTTPClient) *Downloader {
	d := new(Downloader)
	d.channel = name
	d.cache = lru.New(maxCacheEntries)
	d.httpClient = client
	d.dir = workdir
	if len(telegramToken) == 0 {
		log.Fatalln("no token provided")
	}
	d.notifier = telegram.New(telegramToken, chatRoom)

	return d
}
