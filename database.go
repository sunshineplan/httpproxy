package main

import (
	"compress/gzip"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sunshineplan/utils/scheduler"
	"github.com/sunshineplan/utils/txt"
)

const timeFormat = time.RFC3339Nano

var database string

var (
	db            sync.Map
	databaseMutex sync.Mutex
)

func init() {
	scheduler.NewScheduler().At(scheduler.AtClock(0, 0, 0)).Do(func(t time.Time) {
		db.Range(func(_, v any) bool {
			record := v.(*record)
			record.today.Add(-record.today.Load())
			if t.Day() == 1 {
				record.monthly.Add(-record.monthly.Load())
			}
			return true
		})
	})
}

type record struct {
	today, monthly, total atomic.Int64
}

func (r *record) add(n int64) {
	r.today.Add(n)
	r.monthly.Add(n)
	r.total.Add(n)
}

func store(user string, today, monthly, total int64) {
	v := new(record)
	v.today.Store(today)
	v.monthly.Store(monthly)
	v.total.Store(total)
	db.Store(user, v)
}

func count(user string, n int64) {
	if v, ok := db.Load(user); ok {
		v.(*record).add(n)
	} else {
		store(user, n, n, n)
	}
}

func parseDatabase(rows []string) {
	if len(rows) == 0 {
		return
	}
	t, err := time.Parse(timeFormat, rows[0])
	if err != nil {
		errorLogger.Print(err)
		return
	}
	t, now := t.Truncate(24*time.Hour), time.Now().Truncate(24*time.Hour)
	for _, row := range rows[1:] {
		s := strings.Split(row, ":")
		var today, monthly, total int64
		total, err = strconv.ParseInt(s[3], 10, 64)
		if err != nil {
			errorLogger.Println(row, err)
			continue
		}
		if t.Year() == now.Year() && t.Month() == now.Month() {
			monthly, err = strconv.ParseInt(s[2], 10, 64)
			if err != nil {
				errorLogger.Println(row, err)
				continue
			}
		}
		if t == now {
			today, err = strconv.ParseInt(s[1], 10, 64)
			if err != nil {
				errorLogger.Println(row, err)
				continue
			}
		}
		store(s[0], today, monthly, total)
	}
}

func saveDatabase() {
	databaseMutex.Lock()
	defer databaseMutex.Unlock()

	f, err := os.CreateTemp("", "")
	if err != nil {
		errorLogger.Print(err)
		return
	}
	zw := gzip.NewWriter(f)
	fmt.Fprintln(zw, time.Now().Format(timeFormat))

	secretsMutex.Lock()
	defer secretsMutex.Unlock()

	for user := range accounts {
		if v, ok := db.Load(user.name); ok {
			record := v.(*record)
			fmt.Fprintf(zw, "%s:%d:%d:%d\n", user.name, record.today.Load(), record.monthly.Load(), record.total.Load())
		}
	}
	zw.Close()
	f.Close()
	os.Rename(f.Name(), database)
}

func initDatabase() {
	if *debug {
		accessLogger.Println("database:", database)
	}
	if f, err := os.Open(database); err == nil {
		defer f.Close()
		if zr, err := gzip.NewReader(f); err == nil {
			defer zr.Close()
			rows, err := txt.ReadAll(zr)
			if err != nil {
				errorLogger.Print(err)
			} else {
				parseDatabase(rows)
			}
		} else {
			errorLogger.Print(err)
		}
	} else {
		errorLogger.Print(err)
	}
	scheduler.NewScheduler().At(scheduler.AtMinute(0)).Do(func(_ time.Time) { saveDatabase() })
}
