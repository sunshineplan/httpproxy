package main

import (
	"io"
	"log"
	"os"
)

var accessLogger = log.Default()
var errorLogger = log.Default()

func initLogger() {
	if *accesslog != "" {
		f, err := os.OpenFile(*accesslog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Println("failed to open access log file:", err)
			if !*debug {
				accessLogger.SetOutput(io.Discard)
			}
		} else {
			accessLogger.SetOutput(f)
		}
	} else if !*debug {
		accessLogger.SetOutput(io.Discard)
	}

	if *errorlog != "" {
		f, err := os.OpenFile(*errorlog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Println("failed to open access log file:", err)
			if !*debug {
				errorLogger.SetOutput(io.Discard)
			}
		} else {
			log.SetOutput(io.MultiWriter(os.Stderr, f))
			errorLogger.SetOutput(f)
		}
	} else if !*debug {
		errorLogger.SetOutput(io.Discard)
	}
}
