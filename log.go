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
	if *accesslog != "" || !*debug {
		accessLogger = log.New(*accesslog, "", log.LstdFlags)
	}
	if *errorlog != "" {
		errorLogger = log.New(*errorlog, "", log.LstdFlags)
		errorLogger.SetExtra(os.Stderr)
	} else if !*debug {
		errorLogger = log.New("", "", 0)
	}
	if *debug {
		accessLogger.SetLevel(slog.LevelDebug)
		errorLogger.SetLevel(slog.LevelDebug)
		accessLogger.Debug("accesslog: " + *accesslog)
		errorLogger.Debug("errorlog: " + *errorlog)
	}
	server.ErrorLog = errorLogger.Logger
}
