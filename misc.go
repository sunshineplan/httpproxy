package main

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/time/rate"
)

var (
	notAllow     = newSometimes(time.Minute)
	authRequired = newSometimes(time.Minute)
	authFailed   = newSometimes(time.Minute)
)

func newSometimes(interval time.Duration) rate.Sometimes { return rate.Sometimes{Interval: interval} }

func watchFile(file string, fnChange, fnRemove func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = w.Add(filepath.Dir(file)); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case err, ok := <-w.Errors:
				if !ok {
					accessLogger.Println(file, "watcher closed")
					return
				}
				errorLogger.Print(err)
			case event, ok := <-w.Events:
				if !ok {
					accessLogger.Println(file, "watcher closed")
					return
				}
				if event.Name == file {
					accessLogger.Println(file, "operation:", event.Op)
					switch {
					case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
						fnChange()
					case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
						fnRemove()
					}
				}
			}
		}
	}()

	return nil
}
