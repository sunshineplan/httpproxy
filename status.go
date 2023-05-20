package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/txt"
	"github.com/sunshineplan/utils/unit"
)

var c = cache.New(true)
var start time.Time

func set(key string, n int64, d time.Duration) {
	v, ok := c.Get(key)
	if ok {
		c.Set(key, v.(*atomic.Int64).Add(n), d, nil)
	} else {
		v := new(atomic.Int64)
		v.Store(n)
		c.Set(key, v, d, nil)
	}
}

func count(user string, count int64) {
	set(user, count, 0)
	set(time.Now().Format("2006-01")+user, count, 31*24*time.Hour)
	set(time.Now().Format("2006-01-02")+user, count, 24*time.Hour)
}

type statusResult struct {
	user                  string
	today, monthly, total unit.ByteSize
}

var emptyStatus statusResult

func (res statusResult) String(length [3]int) string {
	return fmt.Sprint(
		res.user, strings.Repeat(" ", length[0]-len(res.user)+3),
		res.today, strings.Repeat(" ", length[1]-len(res.today.String())+3),
		res.monthly, strings.Repeat(" ", length[2]-len(res.monthly.String())+3),
		res.total,
	)
}

func getStatus(user string) (res statusResult) {
	var total, monthly, today unit.ByteSize
	v, ok := c.Get(user)
	if ok {
		total = unit.ByteSize(v.(*atomic.Int64).Load())
	}
	v, ok = c.Get(time.Now().Format("2006-01") + user)
	if ok {
		monthly = unit.ByteSize(v.(*atomic.Int64).Load())
	}
	v, ok = c.Get(time.Now().Format("2006-01-02") + user)
	if ok {
		today = unit.ByteSize(v.(*atomic.Int64).Load())
	}

	if total+monthly+today != 0 {
		res = statusResult{user, today, monthly, total}
	}

	return
}

func writeStatus(w *txt.Writer) {
	var res []statusResult
	if status := getStatus("anonymous"); status != emptyStatus {
		res = append(res, status)
	}
	secretsMutex.Lock()
	for user := range accounts {
		if status := getStatus(user.name); status != emptyStatus {
			res = append(res, status)
		}
	}
	secretsMutex.Unlock()

	sort.Slice(res, func(i, j int) bool {
		if res[i].today == res[j].today {
			if res[i].monthly == res[j].monthly {
				return res[i].today > res[j].today
			}
			return res[i].monthly > res[j].monthly
		}
		return res[i].today > res[j].today
	})

	length := [3]int{4, 5, 7}
	for _, i := range res {
		if l := len(i.user); l > length[0] {
			length[0] = l
		}
		if l := len(i.today.String()); l > length[1] {
			length[1] = l
		}
		if l := len(i.monthly.String()); l > length[2] {
			length[2] = l
		}
	}

	fmt.Fprint(
		w,
		"user", strings.Repeat(" ", length[0]-1),
		"today", strings.Repeat(" ", length[1]-2),
		"monthly", strings.Repeat(" ", length[2]-4),
		"total\n",
	)
	for _, i := range res {
		w.WriteLine(i.String(length))
	}
}

func saveStatus() {
	f, err := os.Create(*status)
	if err != nil {
		errorLogger.Print(err)
		return
	}
	defer f.Close()

	w := txt.NewWriter(f)
	defer w.Flush()

	w.WriteLine("Start time: " + start.Format("2006-01-02 15:04:05"))
	w.WriteLine("Last update: " + time.Now().Format("2006-01-02 15:04:05"))
	w.WriteLine(fmt.Sprintf(
		"\nThroughput:\nSend: %s   Receive: %s\n",
		unit.ByteSize(server.WriteCount()),
		unit.ByteSize(server.ReadCount()),
	))
	writeStatus(w)
}

func initStatus() {
	if _, err := os.Stat(*status); err == nil {
		if err := keepStatus(0); err != nil {
			errorLogger.Print(err)
			return
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		errorLogger.Print(err)
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
