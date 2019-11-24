package streaming

import (
	"github.com/pkg/errors"
	"os"
)

const (
	Stopped = 0
	Running = 1

	NormalStream = 1
)

var (
	StreamBucket = []byte("stream")
	ConfigBucket = []byte("config")
)

var ApplicationJson = "application/json"
var ErrorInvalidUri = errors.New("invalid URI")
var ErrorDuplicatedStream = errors.New("duplicated stream")
var ErrorStreamNotFound = errors.New("stream not found")

type Result struct {
	Error string `json:"error"`
}

func NewResult(err error) *Result {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return &Result{
		Error: errMsg,
	}
}

type LiveVideoFile struct {
	File os.FileInfo
	Ext  string
	Dir  string
}

func NewLiveVideoFile(f os.FileInfo, ext, dir string) *LiveVideoFile {
	return &LiveVideoFile{
		File: f,
		Ext:  ext,
		Dir:  dir,
	}
}

type VideoFile struct {
	Name     string
	Duration float64
}
