package main

import (
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
		for event := range w.Events {
			accessLogger.Println(file, "operation:", event.Op)
			switch {
			case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
				accessLogger.Println(file, "changed")
				fnChange()
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				accessLogger.Println(file, "removed")
				fnRemove()
			case event.Has(fsnotify.Chmod):

			default:
				accessLogger.Println(file, "watcher closed")
				return
			}
		}
	}()

	return nil
}
