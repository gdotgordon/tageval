package tageval

import (
	"fmt"
	"io"
	"log"
)

// Constants for the various log levels in increasing verbosity.
const (
	logOff logLevel = iota
	logErr
	logWarn
	logInfo
	logTrace
)

// LogLevel is the type for logging thresholds.
type logLevel int

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
type logger struct {
	Level    logLevel
	traceLog *log.Logger
	infoLog  *log.Logger
	warnLog  *log.Logger
	errLog   *log.Logger
}

// NewLogger creates a new logger that writes to the specifed writer,
// and uses the supplied logging level.
func newLogger(writer io.Writer, level logLevel) *logger {
	traceHandle := io.Discard
	infoHandle := io.Discard
	warningHandle := io.Discard
	errorHandle := io.Discard

	// Was I just looking for a valid use case for "fallthough"?
	// I think this may be one of those cases where it makes sense.
	switch level {
	case logTrace:
		traceHandle = writer
		fallthrough
	case logInfo:
		infoHandle = writer
		fallthrough
	case logWarn:
		warningHandle = writer
		fallthrough
	case logErr:
		errorHandle = writer
	case logOff:
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

	return &logger{
		level,
		traceLog,
		infoLog,
		warningLog,
		errorLog,
	}
}

// Trace logs messages at or above level Trace.
func (lg *logger) trace(fmtmsg string, a ...interface{}) {
	if lg.Level >= logTrace {
		lg.traceLog.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Info logs messages at or above level Info.
func (lg *logger) info(fmtmsg string, a ...interface{}) {
	if lg.Level >= logInfo {
		lg.infoLog.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Warning logs messages at or above level Warning.
func (lg *logger) warn(fmtmsg string, a ...interface{}) {
	if lg.Level >= logWarn {
		lg.warnLog.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}

// Error logs messages at or above level Error.
func (lg *logger) err(fmtmsg string, a ...interface{}) {
	if lg.Level >= logErr {
		lg.errLog.Output(2, fmt.Sprintf(fmtmsg, a...))
	}
}
