package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sunshineplan/utils/cache"
	"github.com/sunshineplan/utils/counter"
	"github.com/sunshineplan/utils/scheduler"
	"github.com/sunshineplan/utils/txt"
)

const timeFormat = time.RFC3339Nano

var recordFile string

var recordMap = cache.NewMap[string, *record]()

func init() {
	scheduler.NewScheduler().At(scheduler.AtClock(0, 0, 0)).Do(func(t time.Time) {
		recordMap.Range(func(_ string, v *record) bool {
			v.today.Add(-v.today.Load())
			if t.Day() == 1 {
				v.monthly.Add(-v.monthly.Load())
			}
			return true
		})
	})
}

type record struct {
	today, monthly, total counter.Counter
}

func (r *record) writer(w io.Writer) io.Writer {
	return r.today.AddWriter(r.monthly.AddWriter(r.total.AddWriter(w)))
}

func store(user string, today, monthly, total int64) *record {
	v := new(record)
	v.today.Add(today)
	v.monthly.Add(monthly)
	v.total.Add(total)
	recordMap.Store(user, v)
	return v
}

func count(user string, w io.Writer) io.Writer {
	if user == "" {
		return w
	}
	if v, ok := recordMap.Load(user); ok {
		return v.writer(w)
	}
	return store(user, 0, 0, 0).writer(w)
}

func parseRecord(rows []string) {
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

func saveRecord(base *Base) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		errorLogger.Print(err)
		return
	}
	zw := gzip.NewWriter(f)
	fmt.Fprintln(zw, time.Now().Format(timeFormat))

	base.accounts.Range(func(a account, _ *limit) bool {
		if v, ok := recordMap.Load(a.name); ok {
			fmt.Fprintf(zw, "%s:%d:%d:%d\n", a.name, v.today.Load(), v.monthly.Load(), v.total.Load())
		}
		return true
	})

	zw.Close()
	f.Close()
	os.Rename(f.Name(), recordFile)
}

func initRecord(base *Base) {
	accessLogger.Debug("record file: " + recordFile)
	if f, err := os.Open(recordFile); err == nil {
		defer f.Close()
		if zr, err := gzip.NewReader(f); err == nil {
			defer zr.Close()
			parseRecord(txt.ReadAll(zr))
		} else {
			errorLogger.Print(err)
		}
	} else {
		errorLogger.Print(err)
	}
	scheduler.NewScheduler().At(scheduler.AtMinute(0)).Do(func(_ time.Time) { saveRecord(base) })
}
