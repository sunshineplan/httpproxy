package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/txt"
	"github.com/sunshineplan/utils/unit"
)

var c = cache.New(true)
var statusMutex sync.Mutex

var fmtBytes = unit.FormatBytes

func set(key string, n uint64, d time.Duration) {
	v, ok := c.Get(key)
	if ok {
		c.Set(key, v.(uint64)+n, d, nil)
	} else {
		c.Set(key, n, d, nil)
	}
}

func count(user string, count uint64) {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	set(user, count, 0)
	set(time.Now().Format("2006-01")+user, count, 31*24*time.Hour)
	set(time.Now().Format("2006-01-02")+user, count, 24*time.Hour)
}

func getStatus(user string) string {
	var total, monthly, today uint64
	v, ok := c.Get(user)
	if ok {
		total = v.(uint64)
	}
	v, ok = c.Get(time.Now().Format("2006-01") + user)
	if ok {
		monthly = v.(uint64)
	}
	v, ok = c.Get(time.Now().Format("2006-01-02") + user)
	if ok {
		today = v.(uint64)
	}

	if total+monthly+today != 0 {
		return fmt.Sprintf("%s   %s   %s", fmtBytes(today), fmtBytes(monthly), fmtBytes(total))
	}

	return ""
}

func saveStatus() {
	f, err := os.Create(*status)
	if err != nil {
		log.Print(err)
		return
	}
	defer f.Close()

	w := txt.NewWriter(f)

	w.WriteLine("Last update: " + time.Now().Format("2006-01-02 15:04:05"))
	w.WriteLine(fmt.Sprintf("\nThroughput:\nSend: %s   Receive: %s\n", fmtBytes(server.WriteCount()), fmtBytes(server.ReadCount())))
	w.WriteLine("[user]: [today]   [monthly]   [total]")

	status := getStatus("anonymous")
	if status != "" {
		w.WriteLine(fmt.Sprintf("anonymous: %s", status))
	}

	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	for user := range accounts {
		status := getStatus(user)
		if status != "" {
			w.WriteLine(fmt.Sprintf("%s: %s", user, status))
		}
	}
	w.Flush()
}

func initStatus() {
	saveStatus()

	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			saveStatus()
		}
	}()
}
