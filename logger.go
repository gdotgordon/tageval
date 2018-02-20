package tageval

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
)

// Constants for the various log levels in increasing verbosity.
const (
	Off LogLevel = iota
	Error
	Warning
	Info
	Trace
)

// LogLevel is the type for logging thresholds.
type LogLevel int

// Logger is a simple logger with configurable level filter that
// avoids uneeded string construction.
// Inspired in part by:
// https://www.ardanlabs.com/blog/2013/11/using-log-package-in-go.html
// and also the Java 8 addition of lambdas to the logging API.
//
// Honestly, I'm starting to see Dave Chaney's point of view that
// loggers don't help much, except during development.  And requiring
// a specific logger in a library seeems problematic.  Hence the simple
// self-contained logger, that should have logging level set to "OFF",
// unless trying to track a problem down.
type Logger struct {
	Level LogLevel
	trace *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
}

// NewLogger creates a new logger that writes to the specifed writer,
// and uses the supplied logging level.
func NewLogger(writer io.Writer, level LogLevel) *Logger {
	traceHandle := ioutil.Discard
	infoHandle := ioutil.Discard
	warningHandle := ioutil.Discard
	errorHandle := ioutil.Discard

	// Was I just looking for a valid use case for "fallthough"?
	// I think this may be one of those cases.
	switch level {
	case Trace:
		traceHandle = writer
		fallthrough
	case Info:
		infoHandle = writer
		fallthrough
	case Warning:
		warningHandle = writer
		fallthrough
	case Error:
		errorHandle = writer
	case Off:
	}

	traceLog := log.New(traceHandle,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	infoLog := log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	warningLog := log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	errorLog := log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		level,
		traceLog,
		infoLog,
		warningLog,
		errorLog,
	}
}

// Trace logs messages at or above level Trace.
func (lg *Logger) Trace(fmtmsg string, a ...interface{}) {
	if lg.Level >= Trace {
		lg.trace.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Info logs messages at or above level Info.
func (lg *Logger) Info(fmtmsg string, a ...interface{}) {
	if lg.Level >= Info {
		lg.info.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Warning logs messages at or above level Warning.
func (lg *Logger) Warning(fmtmsg string, a ...interface{}) {
	if lg.Level >= Warning {
		lg.warn.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Error logs messages at or above level Error.
func (lg *Logger) Error(fmtmsg string, a ...interface{}) {
	if lg.Level >= Error {
		lg.err.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}
