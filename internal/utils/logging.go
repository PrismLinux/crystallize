package utils

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

// InitLogging initializes the logging system
func InitLogging(verbosity int) {
	logger = logrus.New()
	logger.SetOutput(os.Stdout)

	// Set log level based on verbosity
	switch verbosity {
	case 0:
		logger.SetLevel(logrus.InfoLevel)
	case 1:
		logger.SetLevel(logrus.DebugLevel)
	default:
		logger.SetLevel(logrus.TraceLevel)
	}

	// Custom formatter
	logger.SetFormatter(&CustomFormatter{})
}

// CustomFormatter implements a custom log formatter
type CustomFormatter struct{}

// Format formats the log entry
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("15:04:05")

	// Color coding for levels using the fatih/color library
	var levelColor string
	switch entry.Level {
	case logrus.ErrorLevel:
		levelColor = color.New(color.FgRed).Sprint("ERROR")
	case logrus.WarnLevel:
		levelColor = color.New(color.FgYellow).Sprint("WARN")
	case logrus.InfoLevel:
		levelColor = color.New(color.FgGreen).Sprint("INFO")
	case logrus.DebugLevel:
		levelColor = color.New(color.FgCyan).Sprint("DEBUG")
	default:
		levelColor = entry.Level.String()
	}

	return fmt.Appendf(nil, "[ %s ] %s %s\n", levelColor, timestamp, entry.Message), nil
}

// Logging functions

func LogInfo(format string, args ...any) {
	if logger != nil {
		logger.Infof(format, args...)
	}
}

func LogDebug(format string, args ...any) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}

func LogWarn(format string, args ...any) {
	if logger != nil {
		logger.Warnf(format, args...)
	}
}

func LogError(format string, args ...any) {
	if logger != nil {
		logger.Errorf(format, args...)
	}
}

func LogTrace(format string, args ...any) {
	if logger != nil {
		logger.Tracef(format, args...)
	}
}

// Sprintf is a convenience wrapper around fmt.Sprintf
func Sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// NewError creates a new error with the given message
func NewError(message string) error {
	return fmt.Errorf("%s", message)
}

// NewErrorf creates a new formatted error
func NewErrorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
