package main

import (
	"log"
	"strings"
	"sync"
	"unicode"

	"github.com/fsnotify/fsnotify"
	"github.com/sunshineplan/utils/txt"
)

var secretsMutex sync.Mutex
var accounts map[string][]string

func initSecrets() {
	if *secrets != "" {
		rows, err := txt.ReadFile(*secrets)
		if err != nil {
			log.Println("failed to load secrets file:", err)
		}
		parseSecrets(rows, true)

		w, err := fsnotify.NewWatcher()
		if err != nil {
			log.Print(err)
			return
		}
		if err = w.Add(*secrets); err != nil {
			log.Print(err)
			return
		}

		go func() {
			for {
				event, ok := <-w.Events
				if !ok {
					return
				}

				switch event.Op.String() {
				case "WRITE", "CREATE":
					rows, _ := txt.ReadFile(*secrets)
					parseSecrets(rows, false)
				case "REMOVE", "RENAME":
					parseSecrets(nil, false)
				}
			}
		}()
	}
}

func parseSecrets(s []string, record bool) {
	m := make(map[string][]string)
	for _, row := range s {
		if i := strings.IndexRune(row, '#'); i != -1 {
			row = row[:i]
		}
		fields := strings.FieldsFunc(row, func(c rune) bool {
			return unicode.IsSpace(c) || c == ':'
		})
		if l := len(fields); l == 0 {
			continue
		} else if l != 2 {
			if record {
				log.Println("invalid secret:", row)
			}
			continue
		}
		m[fields[0]] = append(m[fields[0]], fields[1])
	}

	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	accounts = m
}

func hasAccount(user, pass string) bool {
	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	if v, ok := accounts[user]; ok {
		for _, i := range v {
			if i == pass {
				return true
			}
		}
	}

	return false
}
