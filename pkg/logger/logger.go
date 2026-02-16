package logger

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var defaultLogger = &Logger{
	level: LevelInfo,
}

type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	level Level
}

var globalLoggerPtr atomic.Pointer[Logger]

func Init(level Level) {
	l := &Logger{level: level}
	globalLoggerPtr.Store(l)
}

func (l *Logger) shouldLog(level Level) bool {
	return level >= l.level
}

func (l *Logger) logWithLevel(level Level, format string, v ...any) {
	if !l.shouldLog(level) {
		return
	}
	msg := fmt.Sprintf("%s [%s] %s\n",
		time.Now().Format("2006/01/02 15:04:05"),
		level.String(),
		fmt.Sprintf(format, v...))

	if level >= LevelWarn {
		os.Stderr.WriteString(msg)
	} else {
		os.Stdout.WriteString(msg)
	}
}

func (l *Logger) Debug(format string, v ...any) { l.logWithLevel(LevelDebug, format, v...) }
func (l *Logger) Info(format string, v ...any)  { l.logWithLevel(LevelInfo, format, v...) }
func (l *Logger) Warn(format string, v ...any)  { l.logWithLevel(LevelWarn, format, v...) }
func (l *Logger) Error(format string, v ...any) { l.logWithLevel(LevelError, format, v...) }

func getLogger() *Logger {
	if gl := globalLoggerPtr.Load(); gl != nil {
		return gl
	}
	return defaultLogger
}

func Debug(format string, v ...any) { getLogger().Debug(format, v...) }
func Info(format string, v ...any)  { getLogger().Info(format, v...) }
func Warn(format string, v ...any)  { getLogger().Warn(format, v...) }
func Error(format string, v ...any) { getLogger().Error(format, v...) }
