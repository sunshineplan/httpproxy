package main

import (
	"log"
	"net/netip"
	"sync"

	"github.com/sunshineplan/utils/txt"
)

var (
	whitelistMutex sync.Mutex
	allows         []allow
)

type allow string

func isValidAllow(s string) (allow, bool) {
	if _, err := netip.ParseAddr(s); err == nil {
		return allow(s), true
	} else if _, err := netip.ParsePrefix(s); err == nil {
		return allow(s), true
	}
	log.Println("can not parse:", s)
	return "", false
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

func initWhitelist() {
	if *whitelist != "" {
		rows, err := txt.ReadFile(*whitelist)
		if err != nil {
			log.Println("failed to load whitelist file:", err)
		}
		parseWhitelist(rows)

		if err := watcherFile(
			*whitelist,
			func() {
				rows, _ := txt.ReadFile(*whitelist)
				parseWhitelist(rows)
			},
			func() { parseWhitelist(nil) },
		); err != nil {
			log.Print(err)
			return
		}
	}
}

func parseWhitelist(s []string) {
	var res []allow
	for _, i := range s {
		if i, ok := isValidAllow(i); ok {
			res = append(res, i)
		}
	}

	whitelistMutex.Lock()
	defer whitelistMutex.Unlock()

	allows = res
}

func isAllow(remoteAddr string) bool {
	whitelistMutex.Lock()
	defer whitelistMutex.Unlock()

	for _, i := range allows {
		if i.isAllow(remoteAddr) {
			return true
		}
	}
	return false
}
