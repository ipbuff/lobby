package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

var logLevels []LogLevel = []LogLevel{
	logUnknown,
	logVerboseDebug,
	logDebug,
	logInfo,
	logWarning,
	logCritical,
}

func TestLogDVf(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        false,
		logInfo:         false,
		logWarning:      false,
		logCritical:     false,
	}

	expectedString := "[DEBUG VERBOSE] "

	assertLogFuncs(t, expectedPrint, expectedString, LogDVf)

	if err := assertLogString(logVerboseDebug); err != nil {
		t.Errorf("%v", err)
	}
}

func TestLogDf(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        true,
		logInfo:         false,
		logWarning:      false,
		logCritical:     false,
	}

	expectedString := "[DEBUG] "

	assertLogFuncs(t, expectedPrint, expectedString, LogDf)

	if err := assertLogString(logDebug); err != nil {
		t.Errorf("%v", err)
	}
}

func TestLogIf(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        true,
		logInfo:         true,
		logWarning:      false,
		logCritical:     false,
	}

	expectedString := "[INFO] "

	assertLogFuncs(t, expectedPrint, expectedString, LogIf)

	if err := assertLogString(logInfo); err != nil {
		t.Errorf("%v", err)
	}
}

func TestLogWf(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        true,
		logInfo:         true,
		logWarning:      true,
		logCritical:     false,
	}

	expectedString := "[WARNING] "

	assertLogFuncs(t, expectedPrint, expectedString, LogWf)

	if err := assertLogString(logWarning); err != nil {
		t.Errorf("%v", err)
	}
}

func TestLogCf(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        true,
		logInfo:         true,
		logWarning:      true,
		logCritical:     true,
	}

	expectedString := "[CRITICAL] "

	assertLogFuncs(t, expectedPrint, expectedString, LogCf)

	if err := assertLogString(logCritical); err != nil {
		t.Errorf("%v", err)
	}
}

func TestLogUnknown(t *testing.T) {
	expectedPrint := map[LogLevel]bool{
		logUnknown:      true,
		logVerboseDebug: true,
		logDebug:        false,
		logInfo:         false,
		logWarning:      false,
		logCritical:     false,
	}

	expectedString := "[DEBUG VERBOSE] "

	assertLogFuncs(t, expectedPrint, expectedString, LogDVf)

	if err := assertLogString(logUnknown); err != nil {
		t.Errorf("%v", err)
	}
}

func assertLogFuncs(
	t *testing.T,
	expectedPrint map[LogLevel]bool,
	expectedString string,
	logFunc LogFunction,
) {
	testMsg := "Perfectly balanced"
	memLog := bytes.Buffer{}
	log.SetOutput(&memLog)

	prevLogLevel := lobbySettings.logLevel
	defer func() { lobbySettings.logLevel = prevLogLevel }()
	defer log.SetOutput(os.Stderr) // os.Stderr is the default output

	for _, l := range logLevels {
		lobbySettings.logLevel = l
		logFunc(testMsg)
		printed := false
		mll := memLog.Len()
		if mll != 0 {
			printed = true
		}
		shouldPrint, _ := expectedPrint[l]
		if shouldPrint != printed {
			t.Error("printed when it shouldn't")
		} else if mll > 0 {
			if !strings.HasSuffix(memLog.String(), testMsg+"\n") {
				t.Log("log message:", memLog.String())
				t.Error("log message wasn't found")
			}
			if !strings.Contains(memLog.String(), expectedString) {
				t.Log("log message:", memLog.String())
				t.Error("log level header not found")
			}
		}
		memLog.Reset()
	}
}

func assertLogString(l LogLevel) error {
	logLevelString := map[LogLevel]string{
		logVerboseDebug: "VerboseDebug",
		logDebug:        "Debug",
		logInfo:         "Info",
		logWarning:      "Warning",
		logCritical:     "Critical",
		logUnknown:      "Unknown",
	}

	if logLevelString[l] != l.String() {
		return fmt.Errorf("Log Level String didn't match. Expected: '%s', but got '%s'", logLevelString[l], l.String())
	}

	ll := getLogLevel(logLevelString[l])
	if ll != l {
		return fmt.Errorf("Log Level derivation from String didn't match. Expected: '%s', but got '%s'", logLevelString[l], ll.String())
	}

	return nil
}
