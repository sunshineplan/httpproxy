package main

import (
	"net/http"

	"github.com/sunshineplan/httpproxy/auth"
	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/httpsvr"
)

type Base struct {
	*httpsvr.Server
	accounts  *cache.Map[auth.Basic, *limit]
	whitelist *cache.Value[[]allow]
}

func NewBase(host, port string) *Base {
	base := &Base{
		Server:    httpsvr.New(),
		accounts:  cache.NewMap[auth.Basic, *limit](),
		whitelist: cache.NewValue[[]allow](),
	}
	base.Host = host
	base.Port = port
	return base
}

func (base *Base) hasAccount() bool {
	var found bool
	base.accounts.Range(func(a auth.Basic, l *limit) bool {
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

func (base *Base) checkAccount(auth auth.Basic) (found bool, exceeded bool, limit *limit) {
	if limit, ok := base.accounts.Load(auth); !ok {
		return false, false, nil
	} else if limit.daily == 0 && limit.monthly == 0 {
		return true, false, limit
	} else if v, ok := recordMap.Load(auth.Username); ok {
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
		auth, ok := auth.ParseBasic(r)
		if !ok {
			authRequired.Do(func() { accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if found, exceeded, limit := base.checkAccount(auth); !found {
			authFailed.Do(func() { errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if found && exceeded {
			limit.st.Do(func() { accessLogger.Printf("%s[%s] Exceeded traffic limit", r.RemoteAddr, auth.Username) })
			http.Error(w, "exceeded traffic limit", http.StatusForbidden)
			return "", nil, false
		} else {
			return auth.Username, limit.speed, true
		}
	default:
		notAllow.Do(func() { accessLogger.Printf("%s not allow", r.RemoteAddr) })
		http.Error(w, "access not allow", http.StatusForbidden)
		return "", nil, false
	}
}
