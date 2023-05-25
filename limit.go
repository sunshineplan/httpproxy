package main

import (
	"errors"
	"strings"

	"github.com/sunshineplan/utils/unit"
)

type limit struct {
	daily   unit.ByteSize
	monthly unit.ByteSize
}

var emptyLimit limit

func parseLimit(s string) (limit, error) {
	res := strings.Split(s, ":")
	switch len(res) {
	case 1:
		monthly, err := unit.ParseByteSize(res[0])
		if err != nil {
			return emptyLimit, err
		}
		return limit{monthly: monthly}, nil
	case 2:
		daily, err := unit.ParseByteSize(res[0])
		if err != nil {
			return emptyLimit, err
		}
		monthly, err := unit.ParseByteSize(res[1])
		if err != nil {
			return emptyLimit, err
		}
		return limit{daily, monthly}, nil
	default:
		return emptyLimit, errors.New("failed to parse limit")
	}
}

func (limit limit) isExceeded(record *record) bool {
	if limit == emptyLimit {
		return false
	}
	if limit.daily == 0 || record.today.Load() < int64(limit.daily) {
		return record.monthly.Load() >= int64(limit.monthly)
	}
	return record.today.Load() >= int64(limit.daily)
}
