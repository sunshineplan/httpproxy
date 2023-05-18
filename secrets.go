package main

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/sunshineplan/utils/txt"
	"github.com/sunshineplan/utils/unit"
	"golang.org/x/time/rate"
)

var (
	secretsMutex sync.Mutex
	accounts     map[account]unit.ByteSize
	sometimes    map[account]rate.Sometimes
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
	if *secrets != "" {
		rows, err := txt.ReadFile(*secrets)
		if err != nil {
			errorLogger.Println("failed to load secrets file:", err)
		}
		parseSecrets(rows, true)

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
}

func parseSecrets(s []string, record bool) {
	m := make(map[account]unit.ByteSize)
	st := make(map[account]rate.Sometimes)
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
			m[account] = 0
		case 2:
			account, err := parseAccount(fields[0])
			if err != nil {
				if record {
					errorLogger.Println("invalid secret:", fields[0])
				}
				continue
			}
			limit, err := unit.ParseByteSize(fields[1])
			if err != nil {
				if record {
					errorLogger.Println("invalid limit:", fields[1])
				}
				continue
			}
			m[account] = limit
			st[account] = newSometimes(time.Minute)
		}
	}
	accessLogger.Printf("loaded %d accounts", len(m))

	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	accounts = m
	sometimes = st
}

func checkAccount(user, pass string) (hasAccount bool, exceeded bool, st rate.Sometimes) {
	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	if limit, ok := accounts[account{user, pass}]; !ok {
		return false, false, rate.Sometimes{}
	} else if limit == 0 {
		return true, false, rate.Sometimes{}
	} else {
		statusMutex.Lock()
		defer statusMutex.Unlock()

		if today, ok := c.Get(time.Now().Format("2006-01-02") + user); ok {
			return true, today.(int64) >= int64(limit), sometimes[account{user, pass}]
		}
	}
	return true, false, rate.Sometimes{}
}
