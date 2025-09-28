package utils

import (
	"fmt"
	"os"
)

// Crash logs an error message and exits with the specified code
func Crash(msg string, code int) {
	LogError("%s", msg)
	os.Exit(code)
}

// CrashWithFormat formats an error message and exits
func CrashWithFormat(format string, code int, args ...interface{}) {
	Crash(fmt.Sprintf(format, args...), code)
}
