package main

import (
	"errors"
	"strings"
	"time"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/unit"
	"golang.org/x/time/rate"
)

type limit struct {
	daily   unit.ByteSize
	monthly unit.ByteSize
	speed   *limiter.Limiter
	st      *rate.Sometimes
}

func parseLimit(s string) (*limit, error) {
	var lim *limiter.Limiter
	res := strings.Split(s, "|")
	switch len(res) {
	case 1:
		lim = limiter.New(limiter.Inf)
	case 2:
		if s := strings.TrimSpace(res[1]); s == "" {
			lim = limiter.New(limiter.Inf)
		} else {
			bs, err := unit.ParseByteSize(s)
			if err != nil {
				return nil, err
			}
			lim = limiter.New(limiter.Limit(bs))
		}
	default:
		return nil, errors.New("failed to parse limit")
	}
	res = strings.Split(res[0], ":")
	switch len(res) {
	case 1:
		s := strings.TrimSpace(res[0])
		if s == "" {
			return &limit{0, 0, lim, nil}, nil
		}
		monthly, err := unit.ParseByteSize(s)
		if err != nil {
			return nil, err
		}
		return &limit{0, monthly, lim, newSometimes(time.Minute)}, nil
	case 2:
		daily, err := unit.ParseByteSize(res[0])
		if err != nil {
			return nil, err
		}
		monthly, err := unit.ParseByteSize(res[1])
		if err != nil {
			return nil, err
		}
		return &limit{daily, monthly, lim, newSometimes(time.Minute)}, nil
	default:
		return nil, errors.New("failed to parse limit")
	}
}

func (limit limit) isExceeded(record *record) bool {
	if limit.daily == 0 && limit.monthly == 0 {
		return false
	}
	if limit.daily == 0 || record.today.Load() < int64(limit.daily) {
		return record.monthly.Load() >= int64(limit.monthly)
	}
	return record.today.Load() >= int64(limit.daily)
}
