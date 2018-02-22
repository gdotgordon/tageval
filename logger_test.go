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

	lf := []func(*Logger, string, ...interface{}){
		(*Logger).Trace, (*Logger).Info, (*Logger).Warning, (*Logger).Error}

	for _, ltest := range tests {
		for i, fn := range lf {
			bb := new(bytes.Buffer)
			log := NewLogger(bb, ltest.level)
			fn(log, "hello")
			if !strings.HasSuffix(strings.TrimSpace(bb.String()), ltest.expected[i]) {
				t.Fatalf("log produced wrong string: '%s'", bb.String())
			}
		}
	}
}
