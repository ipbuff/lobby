package main

import (
	"log"
	"strings"
)

type LogLevel byte

const (
	logUnknown LogLevel = iota
	logVerboseDebug
	logDebug
	logInfo
	logWarning
	logCritical
)

// returns the string value of the LogLevel
func (ll LogLevel) String() string {
	switch ll {
	case logVerboseDebug:
		return "VerboseDebug"
	case logDebug:
		return "Debug"
	case logInfo:
		return "Info"
	case logWarning:
		return "Warning"
	case logCritical:
		return "Critical"
	}
	return "Unknown"
}

// return the LogLevel from string input
func getLogLevel(s string) LogLevel {
	switch strings.ToLower(s) {
	case "verbosedebug":
		return logVerboseDebug
	case "debug":
		return logDebug
	case "info":
		return logInfo
	case "warning":
		return logWarning
	case "critical":
		return logCritical
	}
	return logUnknown
}

type LogFunction func(string, ...interface{})

// LogDVf prints Verbose Debug logs
func LogDVf(format string, args ...interface{}) {
	if lobbySettings.logLevel <= logVerboseDebug {
		log.Printf("[DEBUG VERBOSE] "+format, args...)
	}
}

// LogDf prints Debug logs
func LogDf(format string, args ...interface{}) {
	if lobbySettings.logLevel <= logDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// LogIf prints Info logs
func LogIf(format string, args ...interface{}) {
	if lobbySettings.logLevel <= logInfo {
		log.Printf("[INFO] "+format, args...)
	}
}

// LogWf prints Warning logs
func LogWf(format string, args ...interface{}) {
	if lobbySettings.logLevel <= logWarning {
		log.Printf("[WARNING] "+format, args...)
	}
}

// LogCf prints Critical logs
func LogCf(format string, args ...interface{}) {
	if lobbySettings.logLevel <= logCritical {
		log.Printf("[CRITICAL] "+format, args...)
	}
}
