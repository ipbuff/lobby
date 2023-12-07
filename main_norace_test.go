//go:build !race
// +build !race

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func assertSignal(s os.Signal) error {
	memLog := bytes.Buffer{}
	log.SetOutput(&memLog)
	defer log.SetOutput(os.Stderr) // os.Stderr is the default output

	expectedPrint := lobbySettings.outro
	wt := 2

	var lbs []*lb
	signalHandler(s, sSig, &lbs)
	time.Sleep(time.Duration(wt) * time.Second)

	if !strings.Contains(memLog.String(), expectedPrint) {
		return fmt.Errorf("%s log level assertion failed", expectedPrint)
	}

	return nil
}

func TestSIGINT(t *testing.T) {
	config := `lb:
`
	testConfigPath := "/tmp/lobby_test_conf.yaml"
	lobbySettings.configFilePath = testConfigPath
	os.WriteFile(testConfigPath, []byte(config), 0400)

	go main()

	wt := 2
	time.Sleep(time.Duration(wt) * time.Second)
	os.Remove(testConfigPath)

	if err := assertSignal(syscall.SIGINT); err != nil {
		t.Errorf("Signal result assertion failed: %v", err)
	}
}

func TestSIGTERM(t *testing.T) {
	config := `lb:
  - engine: testEngine
    targets:
      - name: testReconfig
        protocol: tcp
        port: 8082
        upstream_group:
          name: ug
          distribution: round-robin
          upstreams:
            - name: testUpstream
              host: 1.1.1.1
              port: 80
`
	testConfigPath := "/tmp/lobby_test_conf.yaml"
	lobbySettings.configFilePath = testConfigPath
	os.WriteFile(testConfigPath, []byte(config), 0400)

	go main()

	wt := 2
	time.Sleep(time.Duration(wt) * time.Second)
	os.Remove(testConfigPath)

	if err := assertSignal(syscall.SIGTERM); err != nil {
		t.Errorf("Signal result assertion failed: %v", err)
	}
}
