package main

import (
	"io"
	"net/url"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sunshineplan/limiter"
	"golang.org/x/time/rate"
)

var (
	notAllow     = newSometimes(time.Minute)
	authRequired = newSometimes(time.Minute)
	authFailed   = newSometimes(time.Minute)
)

func newSometimes(interval time.Duration) *rate.Sometimes { return &rate.Sometimes{Interval: interval} }

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

func transfer(dst io.WriteCloser, src io.ReadCloser, user user, lim *limiter.Limiter) {
	defer dst.Close()
	defer src.Close()
	if lim == nil {
		io.Copy(count(user, dst), src)
	} else {
		io.Copy(count(user, lim.Writer(dst)), src)
	}
}

func parseProxy(s string) *url.URL {
	accessLogger.Debug("Parse proxy: " + s)
	u, err := url.Parse(s)
	if err != nil {
		errorLogger.Fatalln("bad proxy address:", s)
	}
	accessLogger.Debug("Proxy ready")
	return u
}
