package streaming

import (
	"github.com/boltdb/bolt"
	"github.com/davecgh/go-spew/spew"
	"github.com/devplayg/hippo"
	"github.com/devplayg/rtsp-stream/common"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

type Server struct {
	engine     *hippo.Engine
	controller *Controller
	manager    *Manager
	addr       string
	//liveDir    string
	//recDir     string
	config *common.Config
}

func NewServer(config *common.Config) *Server {
	spew.Dump(config)
	server := &Server{
		config: config,
		addr:   config.BindAddress,
		//liveDir: config.Storage.LiveDir,
	}

	return server
}

func (s *Server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	srv := &http.Server{
		Handler:      s.controller.router,
		Addr:         s.addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.WithFields(log.Fields{
		"address": s.addr,
	}).Info("[server] listen")
	go func() {
		log.Fatal(srv.ListenAndServe())
	}()

	return nil
}

func (s *Server) Stop() error {
	if err := s.manager.Stop(); err != nil {
		log.Error(err)
	}

	if err := common.DB.Close(); err != nil {
		log.Error(err)
	}

	return nil
}

func (s *Server) SetEngine(e *hippo.Engine) {
	s.engine = e
}

func (s *Server) init() error {

	if err := s.initTimezone(); err != nil {
		return err
	}

	if err := s.initDatabase(); err != nil {
		return err
	}

	if err := s.initDirectories(); err != nil {
		return err
	}

	//if err := s.initStorage(); err != nil {
	//	return err
	//}

	// Set manager
	s.manager = NewManager(s)
	if err := s.manager.start(); err != nil {
		return err
	}

	// Set controller
	s.controller = NewController(s)

	return nil
}

func (s *Server) initDirectories() error {

	if err := hippo.EnsureDir(s.config.Storage.LiveDir); err != nil {
		return err
	}

	if !s.config.Storage.Remote {
		if err := hippo.EnsureDir(s.config.Storage.RecordDir); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) initTimezone() error {
	if len(s.config.Timezone) < 1 {
		common.Loc = time.Local
		return nil
	}

	loc, err := time.LoadLocation(s.config.Timezone)
	if err != nil {
		return err
	}
	common.Loc = loc
	return nil
}

func (s *Server) initDatabase() error {
	dbName := s.engine.Config.Name + ".db"
	db, err := bolt.Open(dbName, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}

	defaultBuckets := [][]byte{common.StreamBucket, common.TransmissionBucket, common.ConfigBucket}
	tx, err := db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, b := range defaultBuckets {
		if _, err := tx.CreateBucketIfNotExists(b); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	common.DB = db
	log.WithFields(log.Fields{
		"db": dbName,
	}).Debug("[server] BoltDB has been loaded")
	return nil
}

func (s *Server) GetDbValue(bucket, key []byte) ([]byte, error) {
	var data []byte
	err := common.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucket)
		data = bucket.Get(key)
		return nil
	})
	return data, err
}

//func (s *Server) initStorage() error {
//	client, err := minio.New(s.config.Storage.Address, s.config.Storage.AccessKey, s.config.Storage.SecretKey, s.config.Storage.UseSSL)
//	if err != nil {
//		log.WithFields(log.Fields{
//			"address":   s.config.Storage.Address,
//			"accessKey": s.config.Storage.AccessKey,
//		}).Error("failed to connect to object storage")
//		return err
//	}
//	common.MinioClient = client
//
//	if len(s.config.Storage.Bucket) > 0 {
//		common.VideoRecordBucket = s.config.Storage.Bucket
//	}
//	return nil
//}

func CreateVideoFileList(name string, files []os.FileInfo, dir string) {

}
