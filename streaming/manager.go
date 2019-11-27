package streaming

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"github.com/devplayg/hippo"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// var StreamsKey = []byte("streams") // will be removed

type Manager struct {
	server         *Server
	streams        sync.Map
	reConnInterval time.Duration
}

func NewManager(server *Server) *Manager {
	return &Manager{
		server:  server,
		streams: sync.Map{}, /* key: id(int64), value: &stream */
	}
}

func (m *Manager) save() error {
	if err := m.saveStreams(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) saveStreams() error {

	return DB.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(StreamBucket)

		// Clear bucket
		c := bucket.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			_ = bucket.Delete(k)
		}

		m.streams.Range(func(k interface{}, v interface{}) bool {
			s := v.(*Stream)
			b, err := json.Marshal(s)
			if err != nil {
				log.Error(err)
				return false
			}
			if err := bucket.Put(Int64ToBytes(s.Id), b); err != nil {
				log.Error(err)
				return false
			}

			return true
		})

		return nil
	})
}

func (m *Manager) load() error {
	return DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(StreamBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var stream Stream
			err := json.Unmarshal(v, &stream)
			if err != nil {
				log.Error(err)
				continue
			}
			m.setStream(&stream, stream.Id)
			m.streams.Store(stream.Id, &stream)
		}

		return nil
	})
}

func (m *Manager) getStreams() []*Stream {
	streams := make([]*Stream, 0)
	m.streams.Range(func(k interface{}, v interface{}) bool {
		s := v.(*Stream)
		s.Active = s.IsActive()
		streams = append(streams, s)
		return true
	})
	return streams
}

func (m *Manager) getStreamById(id int64) *Stream {
	val, ok := m.streams.Load(id)
	if !ok {
		return nil
	}

	return val.(*Stream)
}

func (m *Manager) setStream(stream *Stream, id int64) {
	stream.Id = id
	stream.Hash = GetHashString(stream.Uri)
	stream.CmdType = NormalStream
	stream.LiveDir = filepath.ToSlash(filepath.Join(m.server.liveDir, strconv.FormatInt(stream.Id, 16)))
	stream.RecDir = filepath.ToSlash(filepath.Join(m.server.recDir, strconv.FormatInt(stream.Id, 16)))
	// stream.cmd = GenerateStreamCommand(stream)
}

func (m *Manager) addStream(stream *Stream) error {

	// Check if the stream URI is empty or duplicated
	if m.IsExistUri(stream.Uri) || len(stream.Uri) < 1 {
		return ErrorDuplicatedStream
	}

	// Issue auto-increment ID from database
	err := DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(StreamBucket)

		id, _ := b.NextSequence()
		m.setStream(stream, int64(id))

		buf, err := json.Marshal(stream)
		if err != nil {
			return err
		}

		return b.Put(Int64ToBytes(stream.Id), buf)
	})

	if err == nil {
		m.streams.Store(stream.Id, stream)
	}

	if err := m.save(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) updateStream(stream *Stream) error {

	// Check if the stream URI is empty or duplicated
	if len(stream.Uri) < 1 {
		return ErrorInvalidUri
	}

	err := DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(StreamBucket)

		m.setStream(stream, stream.Id)

		buf, err := json.Marshal(stream)
		if err != nil {
			return err
		}

		return b.Put(Int64ToBytes(stream.Id), buf)
	})

	if err == nil {
		m.streams.Store(stream.Id, stream)
	}

	return err
}

func (m *Manager) deleteStream(id int64) error {
	stream := m.getStreamById(id)
	if stream == nil {
		return ErrorStreamNotFound
	}

	err := m.removeStreamDir(stream)
	if err != nil {
		return err
	}

	m.streams.Delete(id)

	if err := m.saveStreams(); err != nil {
		return err
	}
	return m.save()
}

func (m *Manager) cleanStreamDir(stream *Stream) error {
	err := os.RemoveAll(stream.LiveDir)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) removeStreamDir(stream *Stream) error {
	err := os.RemoveAll(stream.LiveDir)
	if err != nil {
		return err
	}
	err = os.RemoveAll(stream.RecDir)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) createStreamDir(stream *Stream) error {
	if err := hippo.EnsureDir(stream.LiveDir); err != nil {
		return err
	}

	if err := hippo.EnsureDir(stream.RecDir); err != nil {
		return err
	}

	return nil
}

func (m *Manager) IsExistUri(uri string) bool {
	duplicated := false
	hash := GetHashString(uri)

	m.streams.Range(func(k interface{}, v interface{}) bool {
		s := v.(*Stream)
		if s.Hash == hash {
			duplicated = true
			return false
		}

		return true
	})

	return duplicated
}

func (m *Manager) startStreaming(stream *Stream) error {
	if err := m.cleanStreamDir(stream); err != nil {
		log.Warn("failed to clear streaming directories:", err)
	}

	if err := m.createStreamDir(stream); err != nil {
		return err
	}

	if err := stream.start(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) stopStreaming(id int64) error {
	stream := m.getStreamById(id)
	if stream == nil {
		return ErrorStreamNotFound
	}

	if err := stream.stop(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) printStream(stream *Stream) {
	log.Debug("===================================================")
	log.Debugf("id: %d", stream.Id)
	log.Debugf("hash: %d", stream.Hash)
	log.Debugf("uri: %s", stream.Uri)
	log.Debugf("active: %s", stream.Active)
	log.Debugf("recording: %s", stream.Recording)
	log.Debug("===================================================")
}

// NEED STREAM RECONNECTION