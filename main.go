package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var version string
var versionCheck bool

// Hardcoded settings
var lobbySettings = app{
	// Application name
	appName: "Lobby",
	// Local config file path
	configFilePath: "./lobby.conf",
	// System config file path
	systemConfigFilePath: "/etc/lobby/lobby.conf",
	// Max healthcheck timer initial wait in milliseconds
	maxHcTimerInit: 500,
	// Seconds to wait for DNS recheck
	defaultDnsTtl: 25,
	// Number of signal interrupts after which the app just exits without waiting for the graceful shutdown to complete
	sigIntCounterExit: 3,
	// Default log level. Set to one of: Critical / Warning / Info / Debug / VerboseDebug
	logLevel: logInfo,
	// Support message
	supportMsg: "In case you're in need of support make sure to check",
	// Support channel to be printed on errors
	supportChannel: "https://github.com/ipbuff/lobby",
	// Exit message
	outro: "Stopped load balancing traffic",
}

type app struct {
	appName              string
	configFilePath       string
	systemConfigFilePath string
	maxHcTimerInit       int
	defaultDnsTtl        uint32
	sigIntCounterExit    uint8
	logLevel             LogLevel
	supportMsg           string
	supportChannel       string
	outro                string
}

// Global var initializations
var (
	sigIntCounter uint8  = 0                   // Holds count of number of Interrupt signals received
	sSig                 = make(chan struct{}) // Holds the system signal channel
	dl            string                       // Holds the debug level string
)

// errUserPrint logs the received errors and provides any relevant additional information to the user
// It can also be used to sanitize or normalize the error messages if required
func errUserPrint(err error) {
	LogCf("%v", err)

	LogIf("%s %s", lobbySettings.supportMsg, lobbySettings.supportChannel)
}

// signalHandler deals with the received system signals
// SIGHUP
//   - load balancer reconfiguration
//
// SIGINT
//   - load balancer exits gracefully
//   - it has a signal counter to abort the graceful exit and forcefully quit
//   - sigIntCounterExit defines the number of SIGINTs to abort graceful exit and forcefully quit
//
// SIGTERM
//   - load balancer exits gracefully
//   - has no signal counter
func signalHandler(s os.Signal, sCh chan struct{}, lbs *[]*lb) {
	LogCf("Received signal '%s'", fmt.Sprint(s))
	switch s {
	case syscall.SIGHUP:
		LogCf("Reconfiguring")
		if err := lbReconfig(lbs); err != nil {
			LogWf("Reconfiguration failed")
			if errors.Is(err, errLbInit) || errors.Is(err, errLbEngineReconfig) {
				LogIf("Previous configuration was retained")
			} else if errors.Is(err, errLbEngineStart) || errors.Is(err, errLbEngineStop) {
				LogWf("Something went wrong with the reconfiguration. This could mean that the current load balancer runtime might be running with unexpected configuration")
				LogIf("Consider rolling back to the previously well-known healthy config. If the issue persists, increase the logging verbosity for further troubleshooting")
			}
			LogIf("%v", err)
		} else {
			LogIf("Reconfigured successfully")
		}
	case syscall.SIGINT:
		sigIntCounter++
		if sigIntCounter > 1 && sigIntCounter < lobbySettings.sigIntCounterExit {
			LogIf("SIGINT signal counter %d/%d", sigIntCounter, lobbySettings.sigIntCounterExit)
		} else if sigIntCounter == lobbySettings.sigIntCounterExit {
			LogCf("Graceful shutdown interrupted. SIGINT signal counter limit reached. Exiting")
			os.Exit(130)
		} else {
			LogCf("Graceful shutdown initiated. To abort and forcefully quit, send more %d SIGINT's", lobbySettings.sigIntCounterExit)
			sCh <- struct{}{}
		}
	case syscall.SIGTERM:
		LogCf("Graceful shutdown initiated")
		sCh <- struct{}{}
	}
}

// Graceful shutdown procedure
func shutdown(lbs []*lb) {
	for _, l := range lbs {
		// Stop load balancer
		LogCf("Stopping load balancer engine '%s'", l.et.String())
		l.stop()
	}

	LogIf("%s", lobbySettings.outro)
}

func init() {
	flag.StringVar(&lobbySettings.configFilePath, "c", lobbySettings.configFilePath, "define the config file path with: '-c /path/to/config/file.yaml'\n")
	flag.StringVar(&dl, "l", lobbySettings.logLevel.String(), "define the verbosity level with: '-l critical/warning/info/debug/verboseDebug'\n")
	flag.BoolVar(&versionCheck, "v", false, "prints version and exits\n")
}

func main() {
	flag.Parse()

	if *&versionCheck {
		fmt.Printf("%s %s\n", lobbySettings.appName, version)
		os.Exit(0)
	}

	var lbs []*lb // Holds the load balancer engines

	// Welcome message
	LogIf("%s %s", lobbySettings.appName, version)

	lobbySettings.logLevel = getLogLevel(dl)

	LogDf("Initializing load balancer")
	lbs, err := lbInit()
	if err != nil {
		errUserPrint(err)
		LogCf("Load Balancer initialization failed. Exiting")
		os.Exit(1)
	}

	LogDf("Initialization succeeded. Starting load balancer engines")
	for _, l := range lbs {
		if err = l.start(); err != nil {
			errUserPrint(err)
			LogCf("Load Balancer start-up failed. Exiting")
			os.Exit(1)
		}
	}

	LogIf("Traffic being load balanced")

	// Create a channel which waits for a SIGHUP, SIGINT or SIGTERM system signals
	osSigCh := make(chan os.Signal, 1)
	signal.Notify(osSigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	// The system signal channel is dealt through a go routine
	go func() {
		for {
			select {
			case s := <-osSigCh:
				signalHandler(s, sSig, &lbs)
			}
		}
	}()

	// Code continues from here when the shutdown procedure is initiated
	<-sSig

	// Initiate shutdown procedure
	shutdown(lbs)
}
