package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sunshineplan/utils/scheduler"
	"github.com/sunshineplan/utils/unit"
)

type usage struct {
	user                  string
	today, monthly, total unit.ByteSize
}

var emptyUsage usage

func (res usage) String(length [3]int) string {
	return fmt.Sprint(
		res.user, strings.Repeat(" ", length[0]-len(res.user)+3),
		res.today, strings.Repeat(" ", length[1]-len(res.today.String())+3),
		res.monthly, strings.Repeat(" ", length[2]-len(res.monthly.String())+3),
		res.total,
	)
}

func getUsage(user string) (res usage) {
	if v, ok := db.Load(user); ok {
		v := v.(*record)
		res.user = user
		res.today = unit.ByteSize(v.today.Load())
		res.monthly = unit.ByteSize(v.monthly.Load())
		res.total = unit.ByteSize(v.total.Load())
	}
	return
}

func writeUsages(w io.Writer) {
	var res []usage
	if usage := getUsage("anonymous"); usage != emptyUsage {
		res = append(res, usage)
	}
	secretsMutex.Lock()
	for user := range accounts {
		if usage := getUsage(user.name); usage != emptyUsage {
			res = append(res, usage)
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
		fmt.Fprintln(w, i.String(length))
	}
}

var start time.Time

func saveStatus() {
	f, err := os.Create(*status)
	if err != nil {
		errorLogger.Print(err)
		return
	}
	defer f.Close()

	fmt.Fprintln(f, "Start Time:", start.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(f, "Last Update:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Throughput:")
	fmt.Fprintf(f, "Send: %s   Receive: %s\n", unit.ByteSize(server.WriteCount()), unit.ByteSize(server.ReadCount()))
	fmt.Fprintln(f)
	writeUsages(f)
}

func initStatus() {
	if *debug {
		accessLogger.Println("status:", *status)
	}
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
	scheduler.NewScheduler().At(scheduler.Every(time.Minute)).Do(func(_ time.Time) { saveStatus() })
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
