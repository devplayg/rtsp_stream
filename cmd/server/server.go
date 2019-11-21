package main

import (
	"github.com/devplayg/hippo"
	"github.com/devplayg/rtsp-stream/server"
	"github.com/sirupsen/logrus"
)

const (
	appName        = "rtsp-server"
	appDisplayName = "RTSP Stream Server"
	appDescription = "RTSP Stream server"
	appVersion     = "1.0.0"
)

func main() {
	config := &hippo.Config{
		Name:        appName,
		DisplayName: appDisplayName,
		Description: appDescription,
		Version:     appVersion,
		IsService:   true,
		Debug:       true,
		Verbose:     true,
	}

	hippo.InitLogger("", appName, config.Debug, config.Verbose)

	addr := "0.0.0.0:9000"
	liveDir := "e:/data/live/"
	recDir := "e:/data/rec/"

	server := server.NewServer(addr, liveDir, recDir)
	engine := hippo.NewEngine(server, config)
	err := engine.Start()
	if err != nil {
		logrus.Fatal(err)
	}
}
