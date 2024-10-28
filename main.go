package main

import (
	"fmt"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/zhangzqs/curl-go/internal"
)

func main() {
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		PadLevelText:  true,
		FullTimestamp: true,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return "", fmt.Sprintf(" %s:%d", path.Base(frame.File), frame.Line)
		},
		TimestampFormat: "2006-01-02T15:04:05.999Z07:00",
		SortingFunc: func(strings []string) {
			copy(strings, []string{"level", "time", "file", "msg"})
		},
	})

	if err := internal.Execute(); err != nil {
		os.Exit(1)
	}
}
