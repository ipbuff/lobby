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

func assertErrUserPrint(expectedPrint map[LogLevel]string) error {
	memLog := bytes.Buffer{}
	log.SetOutput(&memLog)
	defer log.SetOutput(os.Stderr) // os.Stderr is the default output

	for l, p := range expectedPrint {
		errUserPrint(fmt.Errorf(p))
		if !strings.Contains(memLog.String(), p) {
			return fmt.Errorf("%s log level assertion failed", l)
		}
	}

	return nil
}

func TestErrUserPrint(t *testing.T) {
	expectedPrint := map[LogLevel]string{
		logCritical: "Testing err user print",
		logInfo:     lobbySettings.supportMsg,
	}

	if err := assertErrUserPrint(expectedPrint); err != nil {
		t.Errorf("errUserPrint test failed with failed assertion: %v", err)
	}
}

func TestSIGHUP(t *testing.T) {
	config := `lb:
`
	testConfigPath := "/tmp/lobby_test_conf.yaml"
	lobbySettings.configFilePath = testConfigPath
	os.WriteFile(testConfigPath, []byte(config), 0400)

	var err error
	lbs, err := lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}

	wt := 2
	time.Sleep(time.Duration(wt) * time.Second)
	os.Remove(testConfigPath)

	config = `lb:
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
	os.WriteFile(testConfigPath, []byte(config), 0400)

	signalHandler(syscall.SIGHUP, sSig, &lbs)
	time.Sleep(time.Duration(wt) * time.Second)

	if len(lbs) == 0 {
		t.Errorf("SIGHUP reconfig failed")
	}
	os.Remove(testConfigPath)

	config = `lb:
  - engine: turbo
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
	os.WriteFile(testConfigPath, []byte(config), 0400)

	signalHandler(syscall.SIGHUP, sSig, &lbs)
	time.Sleep(time.Duration(wt) * time.Second)

	if lbs[0].et.String() != "testEngine" {
		t.Errorf("Expected engine type was 'testEngine', but got %v", lbs[0].et.String())
	}

	os.Remove(testConfigPath)

	lbs[0].stop()
}
