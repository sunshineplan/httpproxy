package main

import (
	"errors"
	"strings"

	"github.com/sunshineplan/httpproxy/auth"
	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/txt"
)

func parseAccount(s string) (auth.Basic, error) {
	fields := strings.FieldsFunc(s, func(c rune) bool { return c == ':' })
	if len(fields) != 2 {
		return auth.Basic{}, errors.New("invalid account")
	}
	return auth.Basic{Username: fields[0], Password: fields[1]}, nil
}

func initSecrets(file string) *cache.Map[auth.Basic, *limit] {
	accessLogger.Debug("secrets: " + file)
	accounts := cache.NewMap[auth.Basic, *limit]()
	if rows, err := txt.ReadFile(file); err != nil {
		errorLogger.Println("failed to load secrets file:", err)
	} else {
		parseSecrets(accounts, rows)
	}

	if err := watchFile(
		file,
		func() {
			rows, err := txt.ReadFile(file)
			if err != nil {
				errorLogger.Print(err)
			} else {
				accounts.Clear()
				parseSecrets(accounts, rows)
			}
		},
		accounts.Clear,
	); err != nil {
		errorLogger.Print(err)
	}
	return accounts
}

func parseSecrets(m *cache.Map[auth.Basic, *limit], s []string) {
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
				errorLogger.Println("invalid secret:", fields[0])
				continue
			}
			if _, ok := list[account.Username]; !ok {
				m.Store(account, &limit{speed: limiter.New(limiter.Inf)})
				list[account.Username] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.Username)
			}
		case 2:
			account, err := parseAccount(fields[0])
			if err != nil {
				errorLogger.Println("invalid secret:", fields[0])
				continue
			}
			limit, err := parseLimit(fields[1])
			if err != nil {
				errorLogger.Println("invalid limit:", fields[1])
				continue
			}
			if _, ok := list[account.Username]; !ok {
				m.Store(account, limit)
				list[account.Username] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.Username)
			}
		}
	}
	accessLogger.Printf("loaded %d accounts", accountsCount(m))
}

func accountsCount(m *cache.Map[auth.Basic, *limit]) int {
	var n int
	m.Range(func(a auth.Basic, l *limit) bool {
		n++
		return true
	})
	return n
}
