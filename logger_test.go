package tageval

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	type levelTest struct {
		level    LogLevel
		expected []string
	}

	var tests []levelTest
	tests = append(tests, levelTest{Off, []string{"", "", "", ""}})
	tests = append(tests, levelTest{Trace,
		[]string{"hello", "hello", "hello", "hello"}})
	tests = append(tests, levelTest{Info,
		[]string{"", "hello", "hello", "hello"}})
	tests = append(tests, levelTest{Warning,
		[]string{"", "", "hello", "hello"}})
	tests = append(tests, levelTest{Error,
		[]string{"", "", "", "hello"}})

	for _, v := range tests {
		bb := new(bytes.Buffer)
		log := NewLogger(bb, v.level)
		log.Trace("hello")
		if !strings.HasSuffix(strings.TrimSpace(bb.String()), v.expected[0]) {
			t.Fatalf("%v log produced wrong string: '%s'", v.level, bb.String())
		}
		bb = new(bytes.Buffer)
		log = NewLogger(bb, v.level)
		log.Info("hello")
		if !strings.HasSuffix(strings.TrimSpace(bb.String()), v.expected[1]) {
			t.Fatalf("%v log produced wrong string: '%s'", v.level, bb.String())
		}
		bb = new(bytes.Buffer)
		log = NewLogger(bb, v.level)
		log.Warning("hello")
		if !strings.HasSuffix(strings.TrimSpace(bb.String()), v.expected[2]) {
			t.Fatalf("%v log produced wrong string: '%s'", v.level, bb.String())
		}
		bb = new(bytes.Buffer)
		log = NewLogger(bb, v.level)
		log.Error("hello")
		if !strings.HasSuffix(strings.TrimSpace(bb.String()), v.expected[3]) {
			t.Fatalf("%v log produced wrong string: '%s'", v.level, bb.String())
		}
	}
}
