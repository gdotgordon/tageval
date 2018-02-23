package tageval

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	type levelTest struct {
		level    logLevel
		expected []string
	}

	var tests []levelTest
	tests = append(tests, levelTest{logOff, []string{"", "", "", ""}})
	tests = append(tests, levelTest{logTrace,
		[]string{"hello", "hello", "hello", "hello"}})
	tests = append(tests, levelTest{logInfo,
		[]string{"", "hello", "hello", "hello"}})
	tests = append(tests, levelTest{logWarn,
		[]string{"", "", "hello", "hello"}})
	tests = append(tests, levelTest{logErr,
		[]string{"", "", "", "hello"}})

	lf := []func(*logger, string, ...interface{}){
		(*logger).trace, (*logger).info, (*logger).warn, (*logger).err}

	for _, ltest := range tests {
		for i, fn := range lf {
			bb := new(bytes.Buffer)
			log := newLogger(bb, ltest.level)
			fn(log, "hello")
			if !strings.HasSuffix(strings.TrimSpace(bb.String()), ltest.expected[i]) {
				t.Fatalf("log produced wrong string: '%s'", bb.String())
			}
		}
	}
}
