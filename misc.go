package main

import (
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watcherFile(file string, fnChange, fnRemove func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = w.Add(file); err != nil {
		return err
	}

	go func() {
		last := time.Now()
		for event := range w.Events {
			if now := time.Now(); now.Sub(last) > time.Second {
				log.Println(file, "operation:", event.Op)
				last = now
			} else {
				log.Println(file, "ignore operation:", event.Op)
				continue
			}
			switch {
			case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
				fnChange()
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				fnRemove()
			case event.Has(fsnotify.Chmod):

			default:
				log.Println(file, "watcher closed")
				return
			}
		}
	}()

	return nil
}
