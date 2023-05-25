package main

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/sunshineplan/utils/txt"
	"golang.org/x/time/rate"
)

var (
	secretsMutex sync.Mutex
	accounts     map[account]limit
	sometimes    map[account]*rate.Sometimes
)

type account struct {
	name, password string
}

func parseAccount(s string) (account, error) {
	fields := strings.FieldsFunc(s, func(c rune) bool { return c == ':' })
	if len(fields) != 2 {
		return account{}, errors.New("invalid account")
	}
	return account{fields[0], fields[1]}, nil
}

func initSecrets() {
	if *debug {
		accessLogger.Println("secrets:", *secrets)
	}
	if rows, err := txt.ReadFile(*secrets); err != nil {
		errorLogger.Println("failed to load secrets file:", err)
	} else {
		parseSecrets(rows, true)
	}

	if err := watchFile(
		*secrets,
		func() {
			rows, err := txt.ReadFile(*secrets)
			if err != nil {
				errorLogger.Print(err)
			} else {
				parseSecrets(rows, false)
			}
		},
		func() { parseSecrets(nil, false) },
	); err != nil {
		errorLogger.Print(err)
		return
	}
}

func parseSecrets(s []string, record bool) {
	m := make(map[account]limit)
	st := make(map[account]*rate.Sometimes)
	list := make(map[string]struct{})
	for _, row := range s {
		if i := strings.IndexRune(row, '#'); i != -1 {
			row = row[:i]
		}
		fields := strings.Fields(row)
		switch len(fields) {
		case 0:
			continue
		case 1:
			account, err := parseAccount(fields[0])
			if err != nil {
				if record {
					errorLogger.Println("invalid secret:", fields[0])
				}
				continue
			}
			if _, ok := list[account.name]; !ok {
				m[account] = emptyLimit
				list[account.name] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.name)
			}
		case 2:
			account, err := parseAccount(fields[0])
			if err != nil {
				if record {
					errorLogger.Println("invalid secret:", fields[0])
				}
				continue
			}
			limit, err := parseLimit(fields[1])
			if err != nil {
				if record {
					errorLogger.Println("invalid limit:", fields[1])
				}
				continue
			}
			if _, ok := list[account.name]; !ok {
				m[account] = limit
				st[account] = newSometimes(time.Minute)
				list[account.name] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.name)
			}
		}
	}
	accessLogger.Printf("loaded %d accounts", len(m))

	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	accounts = m
	sometimes = st
}
