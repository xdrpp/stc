package stcdetail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Convert a json.Number to an int64 by scanning the textual
// representation of the number.  (The simpler approach of going
// through a double risks losing precision.)
func JsonNumberToI64(n json.Number) (int64, error) {
	val := int64(-1)
	if _, err := fmt.Sscan(n.String(), &val); err != nil {
		return -1, err
	}
	return val, nil
}

// An int64 that marshals and unmarshals as a string in JSON to avoid
// loss of precision (since JSON numbers are floating point).
type JsonInt64 int64

func (i JsonInt64) MarshalText() ([]byte, error) {
	return strconv.AppendInt(nil, int64(i), 10), nil
}

func (i *JsonInt64) UnmarshalText(text []byte) error {
	var err error
	*(*int64)(i), err = strconv.ParseInt(string(text), 10, 64)
	return err
}

// An int64 that marshals and unmarshals to a string in JSON
// containing a fixed-point number 10^{-7} times the value.
type JsonInt64e7 int64

func (i JsonInt64e7) String() string {
	if i == 0 {
		return "0"
	}
	return ScaleFmt(int64(i), 7)
}

func (i JsonInt64e7) MarshalText() ([]byte, error) {
	if i >= 0 {
		return []byte(fmt.Sprintf("%d.%07d", int64(i)/10000000,
			int64(i)%10000000)), nil
	} else {
		return []byte(fmt.Sprintf("%d.%07d", int64(i)/10000000,
			-(int64(i) % 10000000))), nil
	}
}

func (i *JsonInt64e7) UnmarshalText(text []byte) error {
	frac := []byte("0")
	if point := bytes.IndexByte(text, '.'); point >= 0 {
		frac = text[point+1:]
		text = text[:point]
		if len(frac) < 7 {
			frac = append(frac, bytes.Repeat([]byte{'0'}, 7-len(frac))...)
		} else {
			frac = frac[:7]
		}
	}
	if left, err := strconv.ParseInt(string(text), 10, 64); err != nil {
		return err
	} else if right, err := strconv.ParseInt(string(frac), 10, 64); err != nil {
		return err
	} else if left >= 0 {
		*i = JsonInt64e7(left*10000000 + right)
	} else {
		*i = JsonInt64e7(left*10000000 - right)
	}
	return nil
}
