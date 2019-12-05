package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/devplayg/rtsp-stream/common"
	"github.com/grafov/m3u8"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Stream struct {
	Id                 int64                `json:"id"`           // Stream unique ID
	Uri                string               `json:"uri"`          // Stream URL
	Username           string               `json:"username"`     // Stream username
	Password           string               `json:"password"`     // Stream password
	Recording          bool                 `json:"recording"`    // Is recording
	Enabled            bool                 `json:"enabled"`      // Enabled
	Protocol           int                  `json:"protocol"`     // Protocol (HLS, WebM)s
	ProtocolInfo       *common.ProtocolInfo `json:"protocolInfo"` // Protocol info
	UrlHash            string               `json:"urlHash"`      // URL Hash
	cmd                *exec.Cmd            `json:"-"`            // Command
	liveDir            string               `json:"-"`            // Live video directory
	Status             int                  `json:"status"`       // Stream status
	DataRetentionHours int                  `json:"dataRetentionHours"`
	Pid                int                  `json:"pid"`
	assistant          *Assistant
	ctx                context.Context
	cancel             context.CancelFunc
}

func NewStream() *Stream {
	return &Stream{}
}

func (s *Stream) IsActive() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}

	// Check if index file exists
	path := filepath.Join(s.liveDir, s.ProtocolInfo.MetaFileName)
	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	// Check if "index.m3u8" has been updated within the last 8 seconds
	since := time.Now().Sub(file.ModTime()).Seconds()
	//log.Debugf("    [stream-%d] is active? %3.1f", s.Id, since)
	if since > 8.0 {
		return false
	}

	// Check if the .ts file is created continuously
	// wondory

	return true
}

func (s *Stream) StreamUri() string {
	uri := strings.TrimPrefix(s.Uri, "rtsp://")
	return fmt.Sprintf("rtsp://%s:%s@%s", s.Username, s.Password, uri)
}

func (s *Stream) WaitUntilStreamingStarts(startedChan chan<- int, ctx context.Context) {
	count := 1
	for {
		if s.IsActive() {
			startedChan <- count

			// Assistant start
			s.assistant = NewAssistant(s)
			s.assistant.start()
			return
		}
		log.WithFields(log.Fields{
			"count": count,
		}).Debugf("    [stream-%d] waiting until streaming starts", s.Id)
		count++

		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Stream) start() (int, error) {
	s.cmd = GetHlsStreamingCommand(s)
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		defer func() {
			if s.assistant != nil {
				s.assistant.stop()
			}
			s.cancel()
			metaFilePath := filepath.Join(s.liveDir, s.ProtocolInfo.MetaFileName)
			os.Remove(metaFilePath)
			s.Status = common.Stopped
		}()
		err := s.cmd.Run()
		log.WithFields(log.Fields{
			"err": err,
			"pid": GetStreamPid(s),
		}).Debugf("    [stream-%d] process has been terminated", s.Id)
		//s.cmd = nil
	}()

	// Wait until streaming starts
	startedChan := make(chan int)
	go func() {
		s.WaitUntilStreamingStarts(startedChan, s.ctx)
	}()

	// Wait signals
	select {
	case count := <-startedChan:
		//s.Status = Started
		return count, nil
	case <-s.ctx.Done():
		s.stop()
		//s.Status = Failed
		return 0, errors.New("failed or canceled")
	}
}

func (s *Stream) stop() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	err := s.cmd.Process.Kill()
	log.WithFields(log.Fields{
		"uri":    s.Uri,
		"result": err,
	}).Debugf("    [stream-%d] process has been stopped", s.Id)
}

func (s *Stream) makeM3u8Tags(segments []*common.Segment) string {
	size := uint(len(segments))
	playlist, _ := m3u8.NewMediaPlaylist(size, size)
	defer playlist.Close()

	for _, seg := range segments {
		err := playlist.Append(seg.URI, seg.Duration, "")
		if err != nil {
			log.Error(err)
		}
	}
	playlist.Close()
	return playlist.Encode().String()
}

func (s *Stream) getM3u8Segments(date string) []*common.Segment {
	segments := make([]*common.Segment, 0)
	_ = common.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(GetStreamBucketName(s.Id, date))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var seg common.Segment
			err := json.Unmarshal(v, &seg)
			if err != nil {
				log.Error(err)
				continue
			}
			segments = append(segments, &seg)
		}
		return nil
	})
	return segments
}
