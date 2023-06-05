package main

import (
	"errors"
	"strings"

	"github.com/sunshineplan/limiter"
	"github.com/sunshineplan/utils/unit"
)

type limit struct {
	daily   unit.ByteSize
	monthly unit.ByteSize
	speed   *limiter.Limiter
}

func parseLimit(s string) (limit, error) {
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
				return limit{}, err
			}
			lim = limiter.New(limiter.Limit(bs))
		}
	default:
		return limit{}, errors.New("failed to parse limit")
	}
	res = strings.Split(res[0], ":")
	switch len(res) {
	case 1:
		s := strings.TrimSpace(res[0])
		if s == "" {
			return limit{0, 0, lim}, nil
		}
		monthly, err := unit.ParseByteSize(s)
		if err != nil {
			return limit{}, err
		}
		return limit{0, monthly, lim}, nil
	case 2:
		daily, err := unit.ParseByteSize(res[0])
		if err != nil {
			return limit{}, err
		}
		monthly, err := unit.ParseByteSize(res[1])
		if err != nil {
			return limit{}, err
		}
		return limit{daily, monthly, lim}, nil
	default:
		return limit{}, errors.New("failed to parse limit")
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
