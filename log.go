package main

import (
	"log/slog"
	"os"

	"github.com/sunshineplan/utils/log"
)

var (
	accessLogger = log.Default()
	errorLogger  = log.Default()
)

func initLogger() {
	if *debug {
		accessLogger.SetLevel(slog.LevelDebug)
		errorLogger.SetLevel(slog.LevelDebug)
	}
	if *accesslog != "" {
		accessLogger.Debug("accesslog: " + *accesslog)
		accessLogger.SetFile(*accesslog)
	} else if !*debug {
		accessLogger = log.New("", "", 0)
	}

	if *errorlog != "" {
		errorLogger.Debug("errorlog: " + *errorlog)
		errorLogger.SetOutput(*errorlog, os.Stderr)
	} else if !*debug {
		errorLogger = log.New("", "", 0)
	}
	server.ErrorLog = errorLogger.Logger
}
