package main

import (
	"net/netip"

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

func (s allow) isAllow(ip string) bool {
	client, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	if addr, err := netip.ParseAddr(string(s)); err == nil {
		return addr.Compare(client) == 0
	} else if prefix, err := netip.ParsePrefix(string(s)); err == nil {
		return prefix.Contains(client)
	}
	return false
}

func initWhitelist(file string) *cache.Value[[]allow] {
	accessLogger.Debug("whitelist: " + file)
	whitelist := cache.NewValue[[]allow]()
	if rows, err := txt.ReadFile(file); err != nil {
		errorLogger.Println("failed to load whitelist file:", err)
	} else {
		whitelist.Store(parseWhitelist(rows))
	}

	if err := watchFile(
		file,
		func() {
			rows, _ := txt.ReadFile(file)
			whitelist.Store(parseWhitelist(rows))
		},
		func() { whitelist.Store(nil) },
	); err != nil {
		errorLogger.Print(err)
	}
	return whitelist
}

func parseWhitelist(s []string) (allows []allow) {
	for _, i := range s {
		if allow := allow(i); allow.isValid() {
			allows = append(allows, allow)
		}
	}
	return
}
