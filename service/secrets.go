package main

import (
	"errors"
	"strings"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/txt"
	"golang.org/x/net/proxy"
)

type account struct {
	name, password string
}

func (a account) proxyAuth() *proxy.Auth {
	return &proxy.Auth{User: a.name, Password: a.password}
}

func parseAccount(s string) (account, error) {
	fields := strings.FieldsFunc(s, func(c rune) bool { return c == ':' })
	if len(fields) != 2 {
		return account{}, errors.New("invalid account")
	}
	return account{fields[0], fields[1]}, nil
}

func initSecrets(file string) *cache.Map[account, *limit] {
	accessLogger.Debug("secrets: " + file)
	accounts := cache.NewMap[account, *limit]()
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

func parseSecrets(m *cache.Map[account, *limit], s []string) {
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
			if _, ok := list[account.name]; !ok {
				m.Store(account, &limit{speed: limiter.New(limiter.Inf)})
				list[account.name] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.name)
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
			if _, ok := list[account.name]; !ok {
				m.Store(account, limit)
				list[account.name] = struct{}{}
			} else {
				errorLogger.Println("duplicate account name:", account.name)
			}
		}
	}
	accessLogger.Printf("loaded %d accounts", accountsCount(m))
}

func accountsCount(m *cache.Map[account, *limit]) int {
	var n int
	m.Range(func(a account, l *limit) bool {
		n++
		return true
	})
	return n
}
