package main

import (
	"net/http"

	"github.com/sunshineplan/httpproxy/auth"
	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/container"
	"github.com/sunshineplan/utils/httpsvr"
)

type Base struct {
	*httpsvr.Server
	accounts  *container.Map[auth.Basic, *limit]
	whitelist *container.Map[allow, *limit]
}

func NewBase(host, port string) *Base {
	base := &Base{
		Server:    httpsvr.New(),
		accounts:  container.NewMap[auth.Basic, *limit](),
		whitelist: container.NewMap[allow, *limit](),
	}
	base.Host = host
	base.Port = port
	return base
}

func (base *Base) hasAccount() bool {
	var found bool
	base.accounts.Range(func(_ auth.Basic, _ *limit) bool {
		found = true
		return false
	})
	return found
}

func (base *Base) hasWhitelist() bool {
	var found bool
	base.whitelist.Range(func(_ allow, _ *limit) bool {
		found = true
		return false
	})
	return found
}

func (base *Base) isAllow(remoteAddr string) (found bool, a allow, exceeded bool, l *limit) {
	base.whitelist.Range(func(allow allow, limit *limit) bool {
		if allow.isAllow(remoteAddr) {
			found = true
			a = allow
			l = limit
			if v, ok := recordMap.Load(user{string(allow), true}); ok {
				exceeded = limit.isExceeded(v)
			}
			return false
		}
		return true
	})
	return
}

func (base *Base) checkAccount(auth auth.Basic) (found bool, exceeded bool, limit *limit) {
	if limit, ok := base.accounts.Load(auth); !ok {
		return false, false, nil
	} else if limit.daily == 0 && limit.monthly == 0 {
		return true, false, limit
	} else if v, ok := recordMap.Load(user{auth.Username, false}); ok {
		return true, limit.isExceeded(v), limit
	} else {
		return true, false, limit
	}
}

type user struct {
	name      string
	whitelist bool
}

func (base *Base) Auth(w http.ResponseWriter, r *http.Request) (user, *limiter.Limiter, bool) {
	switch hasWhitelist, hasAccount := base.hasWhitelist(), base.hasAccount(); {
	case !hasWhitelist && !hasAccount:
		return user{}, limiter.New(limiter.Inf), true
	case hasWhitelist:
		if found, allow, exceeded, limit := base.isAllow(r.RemoteAddr); found {
			if exceeded {
				limit.st.Do(func() { accessLogger.Printf("%s[%s] Exceeded traffic limit", r.RemoteAddr, allow) })
				http.Error(w, "exceeded traffic limit", http.StatusForbidden)
				return user{}, nil, false
			}
			return user{string(allow), true}, limit.speed, true
		}
		fallthrough
	case hasAccount:
		auth, ok := auth.ParseBasic(r)
		if !ok {
			authRequired.Do(func() { accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return user{}, nil, false
		} else if found, exceeded, limit := base.checkAccount(auth); !found {
			authFailed.Do(func() { errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return user{}, nil, false
		} else if found && exceeded {
			limit.st.Do(func() { accessLogger.Printf("%s[%s] Exceeded traffic limit", r.RemoteAddr, auth.Username) })
			http.Error(w, "exceeded traffic limit", http.StatusForbidden)
			return user{}, nil, false
		} else {
			return user{auth.Username, false}, limit.speed, true
		}
	default:
		notAllow.Do(func() { accessLogger.Printf("%s not allow", r.RemoteAddr) })
		http.Error(w, "access not allow", http.StatusForbidden)
		return user{}, nil, false
	}
}
