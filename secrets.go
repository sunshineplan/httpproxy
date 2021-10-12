package main

import (
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/sunshineplan/utils/txt"
	"github.com/sunshineplan/utils/watcher"
)

var mu sync.Mutex
var accounts map[string][]string

func initSecrets() {
	if *secrets != "" {
		rows, err := txt.ReadFile(*secrets)
		if err != nil {
			log.Println("failed to load secrets file:", err)
			return
		}
		parseSecrets(rows, true)

		w := watcher.New(*secrets, time.Second)
		go func() {
			for {
				<-w.C

				rows, _ := txt.ReadFile(*secrets)
				parseSecrets(rows, false)
			}
		}()
	}
}

func parseSecrets(s []string, debug bool) {
	m := make(map[string][]string)
	for _, row := range s {
		if i := strings.IndexRune(row, '#'); i != -1 {
			row = row[:i]
		}
		fields := strings.FieldsFunc(row, func(c rune) bool {
			return unicode.IsSpace(c) || c == ':'
		})
		if len(fields) != 2 {
			if debug {
				log.Println("invalid secret:", row)
			}
			continue
		}
		m[fields[0]] = append(m[fields[0]], fields[1])
	}

	mu.Lock()
	defer mu.Unlock()

	accounts = m
}

func hasAccount(user, pass string) bool {
	mu.Lock()
	defer mu.Unlock()

	if v, ok := accounts[user]; ok {
		for _, i := range v {
			if i == pass {
				return true
			}
		}
	}

	return false
}
