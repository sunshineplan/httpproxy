package main

import (
	"net/netip"
	"strings"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/txt"
)

type allow string

func (s allow) isValid() bool {
	if _, err := netip.ParseAddr(string(s)); err == nil {
		return true
	} else if _, err := netip.ParsePrefix(string(s)); err == nil {
		return true
	}
	errorLogger.Println("can not parse:", s)
	return false
}

func (allow allow) isAllow(s string) bool {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return false
	}
	if addr, err := netip.ParseAddr(string(allow)); err == nil {
		return addr.Compare(ap.Addr()) == 0
	} else if prefix, err := netip.ParsePrefix(string(allow)); err == nil {
		return prefix.Contains(ap.Addr())
	}
	return false
}

func initWhitelist(file string) *cache.Map[allow, *limit] {
	accessLogger.Debug("whitelist: " + file)
	whitelist := cache.NewMap[allow, *limit]()
	if rows, err := txt.ReadFile(file); err != nil {
		errorLogger.Println("failed to load whitelist file:", err)
	} else {
		parseWhitelist(whitelist, rows)
	}

	if err := watchFile(
		file,
		func() {
			rows, err := txt.ReadFile(file)
			if err != nil {
				errorLogger.Print(err)
			} else {
				whitelist.Clear()
				parseWhitelist(whitelist, rows)
			}
		},
		whitelist.Clear,
	); err != nil {
		errorLogger.Print(err)
	}
	return whitelist
}

func parseWhitelist(m *cache.Map[allow, *limit], s []string) {
	list := make(map[allow]struct{})
	for _, row := range s {
		if i := strings.IndexRune(row, '#'); i != -1 {
			row = row[:i]
		}
		fields := strings.Fields(row)
		switch len(fields) {
		case 0:
			continue
		case 1:
			if allow := allow(fields[0]); allow.isValid() {
				if _, ok := list[allow]; !ok {
					m.Store(allow, &limit{speed: limiter.New(limiter.Inf)})
					list[allow] = struct{}{}
				} else {
					errorLogger.Println("duplicate whitelist record:", allow)
				}
			} else {
				errorLogger.Println("invalid whitelist record:", allow)
			}
		case 2:
			allow := allow(fields[0])
			if !allow.isValid() {
				errorLogger.Println("invalid whitelist record:", allow)
				continue
			}
			limit, err := parseLimit(fields[1])
			if err != nil {
				errorLogger.Println("invalid limit:", fields[1])
				continue
			}
			if _, ok := list[allow]; !ok {
				m.Store(allow, limit)
				list[allow] = struct{}{}
			} else {
				errorLogger.Println("duplicate whitelist record:", allow)
			}
		}
	}
	accessLogger.Printf("loaded %d whitelist accounts", len(list))
}
