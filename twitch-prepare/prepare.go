package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	torrent "github.com/anacrolix/torrent/metainfo"
	"github.com/cydev/twitch/downloader"
)

type Video struct {
	Meta           downloader.Metadata
	Filename       string
	OutputFilename string
}

func (_ Video) metaArg(k, v string) string {
	return fmt.Sprintf(`%s=%s`, k, v)
}

var (
	builtinAnnounceList = [][]string{
		{"udp://tracker.openbittorrent.com:80"},
		{"udp://tracker.leechers-paradise.org:6969"},
		{"udp://tracker.coppersurfer.tk:6969"},
		{"udp://open.demonii.com:1337"},
		{"http://retracker.local"},
	}
)

func (v Video) createTorrent() error {
	log.Println("creating torrent")
	f, err := os.Create(v.torrentFilename())
	defer f.Close()
	if err != nil {
		return err
	}
	b := torrent.Builder{}
	b.AddFile(v.OutputFilename)
	for _, group := range builtinAnnounceList {
		b.AddAnnounceGroup(group)
	}
	batch, err := b.Submit()
	if err != nil {
		log.Fatal(err)
	}
	errs, status := batch.Start(f, runtime.NumCPU())
	lastProgress := int64(-1)
	for {
		select {
		case err, ok := <-errs:
			if !ok || err == nil {
				return err
			}
			log.Print(err)
		case bytesDone := <-status:
			progress := 100 * bytesDone / batch.TotalSize()
			if progress != lastProgress {
				log.Printf("%d%%", progress)
				lastProgress = progress
			}
		}
	}
}

func (v Video) getMetadataArgs() (args []string) {
	args = append(args, "-metadata", v.metaArg("title", v.Meta.Title))
	args = append(args, "-metadata", v.metaArg("author", v.Meta.Author))
	args = append(args, "-metadata", v.metaArg("date", v.Meta.Date.Format(time.RFC3339)))
	args = append(args, "-metadata", v.metaArg("copyright", fmt.Sprintf("Copyright %d %s", v.Meta.Date.Year(), v.Meta.Author)))

	return args
}

func (v Video) torrentFilename() string {
	extension := filepath.Ext(v.OutputFilename)
	return strings.Replace(v.OutputFilename, extension, ".torrent", -1)
}

func (v Video) outputFilename() string {
	extension := filepath.Ext(v.Filename)
	return strings.Replace(v.Filename, extension, "-stream.mp4", -1)
}

func (v Video) command() (cmd *exec.Cmd) {
	args := []string{
		"-i", v.Filename,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
	}
	args = append(args, v.getMetadataArgs()...)
	args = append(args, "-movflags", "faststart", v.OutputFilename)
	cmd = exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	return cmd
}

func (v *Video) Prepare() error {
	v.OutputFilename = v.outputFilename()
	cmd := v.command()
	fmt.Print("exec", cmd.Args)
	return cmd.Run()
}

func prepare(filename string) error {
	var metadata downloader.Metadata
	metadataFile, err := os.Open(downloader.GetMetadataFileName(filename))
	if err != nil {
		log.Println("unable to open metadata", err)
	} else {
		defer metadataFile.Close()
		metadata, err = downloader.ReadMetadata(metadataFile)
		if err != nil {
			log.Println("no metadata found for", filename)
		}
	}
	video := Video{
		Meta:     metadata,
		Filename: filename,
	}
	if err := video.Prepare(); err != nil {
		log.Fatalln("prepare failed:", err)
		return err
	}
	if err := video.createTorrent(); err != nil {
		log.Fatalln("torrent creation failed:", err)
		return err
	}
	return os.Rename(filename, fmt.Sprintf("%s.old", filename))
}

func main() {
	fmt.Println("cydev/twitch-prepare")
	if len(os.Args) != 2 {
		return
	}
	name := os.Args[1]
	stat, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	if !stat.IsDir() {
		prepare(name)
		return
	}
	fmt.Println("processing all files in", name)
	files, err := ioutil.ReadDir(name)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.Contains(f.Name(), "-stream.mp4") {
			continue
		}
		if !strings.HasSuffix(f.Name(), "mp4") {
			continue
		}
		fmt.Println(f.Name())
		prepare(f.Name())
	}
}
