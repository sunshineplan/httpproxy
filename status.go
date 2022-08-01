package main

import (
	"fmt"
	"log"
	"os"
	"strings"
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

type statusResult struct {
	user, today, monthly, total string
}

var emptyStatus statusResult

func (res statusResult) String(length [4]int) (output string) {
	output += res.user + strings.Repeat(" ", length[0]-len(res.user)+3)
	output += res.today + strings.Repeat(" ", length[1]-len(res.today)+3)
	output += res.monthly + strings.Repeat(" ", length[2]-len(res.monthly)+3)
	output += res.total + strings.Repeat(" ", length[3]-len(res.total)+3)
	return
}

func getStatus(user string) (res statusResult) {
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
		res = statusResult{user, fmtBytes(today), fmtBytes(monthly), fmtBytes(total)}
	}

	return
}

func writeStatus(w *txt.Writer) {
	res := []statusResult{{"user", "today", "monthly", "total"}}
	if status := getStatus("anonymous"); status != emptyStatus {
		res = append(res, status)
	}
	secretsMutex.Lock()
	for user := range accounts {
		if status := getStatus(user); status != emptyStatus {
			res = append(res, status)
		}
	}
	secretsMutex.Unlock()

	var length [4]int
	for _, i := range res {
		if l := len(i.user); l > length[0] {
			length[0] = l
		}
		if l := len(i.today); l > length[1] {
			length[1] = l
		}
		if l := len(i.monthly); l > length[2] {
			length[2] = l
		}
		if l := len(i.total); l > length[3] {
			length[3] = l
		}
	}

	for _, i := range res {
		w.WriteLine(i.String(length))
	}
}

func saveStatus() {
	f, err := os.Create(*status)
	if err != nil {
		log.Print(err)
		return
	}
	defer f.Close()

	w := txt.NewWriter(f)
	defer w.Flush()

	w.WriteLine("Start time: " + start.Format("2006-01-02 15:04:05"))
	w.WriteLine("Last update: " + time.Now().Format("2006-01-02 15:04:05"))
	w.WriteLine(fmt.Sprintf("\nThroughput:\nSend: %s   Receive: %s\n", fmtBytes(server.WriteCount()), fmtBytes(server.ReadCount())))

	writeStatus(w)
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
