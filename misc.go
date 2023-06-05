package main

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sunshineplan/limiter"
	"golang.org/x/time/rate"
)

var (
	notAllow     = newSometimes(time.Minute)
	authRequired = newSometimes(time.Minute)
	authFailed   = newSometimes(time.Minute)
)

func newSometimes(interval time.Duration) *rate.Sometimes { return &rate.Sometimes{Interval: interval} }

func watchFile(file string, fnChange, fnRemove func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = w.Add(filepath.Dir(file)); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case err, ok := <-w.Errors:
				if !ok {
					accessLogger.Println(file, "watcher closed")
					return
				}
				errorLogger.Print(err)
			case event, ok := <-w.Events:
				if !ok {
					accessLogger.Println(file, "watcher closed")
					return
				}
				if event.Name == file {
					accessLogger.Println(file, "operation:", event.Op)
					switch {
					case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
						fnChange()
					case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
						fnRemove()
					}
				}
			}
		}
	}()

	return nil
}

func auth(w http.ResponseWriter, r *http.Request) (string, *limiter.Limiter, bool) {
	user := "anonymous"
	var pass string
	var ok bool
	if len(accounts) == 0 && len(allows) != 0 && !isAllow(r.RemoteAddr) {
		notAllow.Do(func() { accessLogger.Printf("%s not allow", r.RemoteAddr) })
		http.Error(w, "access not allow", http.StatusForbidden)
		return "", nil, false
	} else if len(accounts) != 0 && !isAllow(r.RemoteAddr) {
		user, pass, ok = parseBasicAuth(r.Header.Get("Proxy-Authorization"))
		if !ok {
			authRequired.Do(func() { accessLogger.Printf("%s Proxy Authentication Required", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if hasAccount, exceeded, lim, sometimes := checkAccount(user, pass); !hasAccount {
			authFailed.Do(func() { errorLogger.Printf("%s Proxy Authentication Failed", r.RemoteAddr) })
			w.Header().Add("Proxy-Authenticate", `Basic realm="HTTP(S) Proxy Server"`)
			http.Error(w, "", http.StatusProxyAuthRequired)
			return "", nil, false
		} else if hasAccount && exceeded {
			sometimes.Do(func() { accessLogger.Printf("%s[%s] Exceeded traffic limit", r.RemoteAddr, user) })
			http.Error(w, "exceeded traffic limit", http.StatusForbidden)
			return "", nil, false
		} else {
			return user, lim, true
		}
	}
	return user, limiter.New(limiter.Inf), true
}

func checkAccount(user, pass string) (hasAccount bool, exceeded bool, lim *limiter.Limiter, st *rate.Sometimes) {
	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	if limit, ok := accounts[account{user, pass}]; !ok {
		return false, false, nil, nil
	} else if limit.daily == 0 && limit.monthly == 0 {
		return true, false, limit.speed, nil
	} else if v, ok := db.Load(user); ok {
		return true, limit.isExceeded(v.(*record)), limit.speed, sometimes[account{user, pass}]
	} else {
		return true, false, limit.speed, sometimes[account{user, pass}]
	}
}
