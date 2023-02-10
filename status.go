package main

import (
	"errors"
	"fmt"
	"io/fs"
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
var start time.Time

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

func (res statusResult) String(length [3]int) string {
	var b strings.Builder
	b.WriteString(res.user)
	b.WriteString(strings.Repeat(" ", length[0]-len(res.user)+3))
	b.WriteString(res.today)
	b.WriteString(strings.Repeat(" ", length[1]-len(res.today)+3))
	b.WriteString(res.monthly)
	b.WriteString(strings.Repeat(" ", length[2]-len(res.monthly)+3))
	b.WriteString(res.total)
	return b.String()
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

	var length [3]int
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
	if _, err := os.Stat(*status); err == nil {
		if err := keepStatus(0); err != nil {
			log.Print(err)
			return
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Print(err)
		return
	}

	start = time.Now()
	saveStatus()

	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			saveStatus()
		}
	}()
}

func keepStatus(n int) (err error) {
	var src string
	if n == 0 {
		src = *status
	} else {
		src = fmt.Sprint(*status, ".", n)
	}
	dst := fmt.Sprint(*status, ".", n+1)
	if n >= *keep {
		return os.Remove(src)
	}
	if _, err = os.Stat(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	} else if errors.Is(err, fs.ErrNotExist) {
		return os.Rename(src, dst)
	} else {
		defer func() {
			err = keepStatus(n)
		}()
		return keepStatus(n + 1)
	}
}
