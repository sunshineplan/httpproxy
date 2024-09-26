package main

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/httpsvr"
)

var base *Base

type Base struct {
	*httpsvr.Server
	accounts  *cache.Map[account, *limit]
	whitelist *cache.Value[[]allow]
}

func NewBase(host, port string) *Base {
	base := &Base{
		Server:    httpsvr.New(),
		accounts:  cache.NewMap[account, *limit](),
		whitelist: cache.NewValue[[]allow](),
	}
	base.Host = host
	base.Port = port
	return base
}

func (base *Base) hasAccount() bool {
	var found bool
	base.accounts.Range(func(a account, l *limit) bool {
		found = true
		return false
	})
	return found
}

func (base *Base) hasWhitelist() bool {
	allows, _ := base.whitelist.Load()
	return len(allows) > 0
}

func (base *Base) isAllow(remoteAddr string) bool {
	if allows, ok := base.whitelist.Load(); ok {
		for _, i := range allows {
			if i.isAllow(remoteAddr) {
				return true
			}
		}
	}
	return false
}

func (base *Base) checkAccount(user, pass string) (found bool, exceeded bool, limit *limit) {
	if limit, ok := base.accounts.Load(account{user, pass}); !ok {
		return false, false, nil
	} else if limit.daily == 0 && limit.monthly == 0 {
		return true, false, limit
	} else if v, ok := recordMap.Load(user); ok {
		return true, limit.isExceeded(v), limit
	} else {
		return true, false, limit
	}
}

func (base *Base) Auth(w http.ResponseWriter, r *http.Request) (string, *limiter.Limiter, bool) {
	switch hasWhitelist, hasAccount := base.hasWhitelist(), base.hasAccount(); {
	case !hasWhitelist && !hasAccount:
		return "anonymous", limiter.New(limiter.Inf), true
	case hasWhitelist && base.isAllow(r.RemoteAddr):
		return "whitelist", limiter.New(limiter.Inf), true
	case hasAccount:
		user, pass, ok := parseBasicAuth(r.Header.Get("Proxy-Authorization"))
		if !ok {
			authRequired.Do(func() { accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if found, exceeded, limit := base.checkAccount(user, pass); !found {
			authFailed.Do(func() { errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if found && exceeded {
			limit.st.Do(func() { accessLogger.Printf("%s[%s] Exceeded traffic limit", r.RemoteAddr, user) })
			http.Error(w, "exceeded traffic limit", http.StatusForbidden)
			return "", nil, false
		} else {
			return user, limit.speed, true
		}
	default:
		notAllow.Do(func() { accessLogger.Printf("%s not allow", r.RemoteAddr) })
		http.Error(w, "access not allow", http.StatusForbidden)
		return "", nil, false
	}
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}
