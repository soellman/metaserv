package main

import (
	"bufio"
	"io"
	"strings"
)

// LineFuncs process a line of data.
// If the line can't be processed, nil is returned
type LineFunc func(line string) interface{}

// AggFuncs aggregate data from LineFuncs into a data structure
type AggFunc func(data interface{}, lineData interface{})

// ComposeReader processes a reader line by line
// using the LineFunc to process each line
// and AggFunc to aggregate into d
func ComposeReader(
	r io.Reader,
	d interface{},
	l LineFunc,
	a AggFunc,
) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		if ldata := l(s.Text()); ldata != nil {
			a(d, ldata)
		}
	}
}

// SplitOnString returns []string
func SplitOnString(sep string) LineFunc {
	return func(line string) interface{} {
		tokens := strings.Split(strings.TrimSpace(line), sep)
		if len(tokens) < 2 {
			return nil
		}
		return tokens
	}
}

// MatchAndRemove matches string and returns all but the string
func MatchAndRemove(key string) LineFunc {
	return func(line string) interface{} {
		if !strings.Contains(line, key) {
			return nil
		}
		return strings.Replace(strings.TrimSpace(line), key, "", 1)
	}
}

func TupleToMap() AggFunc {
	return func(data interface{}, lineData interface{}) {
		line := lineData.([]string)
		m := data.(map[string]string)
		m[line[0]] = line[1]
	}
}

func MapKey(key string) AggFunc {
	return func(data interface{}, lineData interface{}) {
		line := lineData.(string)
		m := data.(map[string]string)
		m[key] = line
	}
}
