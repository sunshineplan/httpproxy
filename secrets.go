package main

import (
	"log"
	"strings"
	"sync"
	"unicode"

	"github.com/sunshineplan/utils/txt"
)

var (
	secretsMutex sync.Mutex
	accounts     map[string][]string
)

func initSecrets() {
	if *secrets != "" {
		rows, err := txt.ReadFile(*secrets)
		if err != nil {
			log.Println("failed to load secrets file:", err)
		}
		parseSecrets(rows, true)

		if err := watcherFile(
			*secrets,
			func() {
				rows, _ := txt.ReadFile(*secrets)
				parseSecrets(rows, false)
			},
			func() { parseSecrets(nil, false) },
		); err != nil {
			log.Print(err)
			return
		}
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
