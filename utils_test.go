package main

import (
	"reflect"
	"strings"
	"testing"
)

var (
	testTupleBlank  = ""
	testTupleBroken = "one"
	testTupleEquals = "one=two"
	testTupleColon  = "one:two"
)

var lineFuncTests = []struct {
	f   LineFunc
	in  string
	out interface{}
}{
	{SplitOnString("="), testTupleBlank, nil},
	{SplitOnString("="), testTupleBroken, nil},
	{SplitOnString("="), testTupleEquals, []string{"one", "two"}},
	{SplitOnString(":"), testTupleColon, []string{"one", "two"}},
}

func TestLineFuncs(t *testing.T) {
	for _, lt := range lineFuncTests {
		if r := lt.f(lt.in); !reflect.DeepEqual(r, lt.out) {
			t.Errorf("LineFunc failed: expected %v, got %v", lt.out, r)
		}
	}
}

var (
	testTuple = []string{"one", "two"}
	testMap   = map[string]string{"one": "two"}
)

var aggFuncTests = []struct {
	a    AggFunc
	data interface{}
	in   interface{}
	out  interface{}
}{
	{TupleToMap(), make(map[string]string), testTuple, testMap},
}

func TestAggFuncs(t *testing.T) {
	for _, at := range aggFuncTests {
		if at.a(at.data, at.in); !reflect.DeepEqual(at.data, at.out) {
			t.Errorf("LineFunc failed: expected %v, got %v", at.out, at.data)
		}
	}
}

var (
	testReaderString = "one=two\nthree=four\nbad\n\n"
	testResultMap    = map[string]string{"one": "two", "three": "four"}
)

func TestComposeReader(t *testing.T) {
	r := strings.NewReader(testReaderString)
	d := make(map[string]string)
	ComposeReader(r, d, SplitOnString("="), TupleToMap())
	if !reflect.DeepEqual(d, testResultMap) {
		t.Errorf("ComposeReader failed: expected %v, got %v", testResultMap, d)
	}
}
