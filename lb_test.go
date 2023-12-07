package main

import (
	"errors"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

var testConfigYaml = `lb: 
  - engine: nftables                            # load balancer engine. only 'nftables' supported for now
    targets: 
      - name: target1                           # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8080                              # target port
        upstream_group: 
          name: ug0                             # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: t1ug0u1                     # upstream name
              host: 8.8.8.8                     # upstream host. IP Address or domain name
              port: 8080                        # upstream port
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
            - name: t1ug0u2                     # upstream name
              host: 8.8.8.8                     # upstream host. IP Address or domain name
              port: 6443                        # upstream port
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 6443                      # health_check port
                start_available: true           # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
            - name: t1ug0u3                     # upstream name
              host: 8.8.8.8                     # upstream host. IP Address or domain name
              port: 8081                        # upstream port
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8081                      # health_check port
                start_available: false          # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
      - name: target2                           # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8081                              # target port
        upstream_group:                         # upstream_group to be used for target
          name: ug1                             # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: t2ug1u1                     # upstream name
              host: 8.8.8.8                     # upstream host. IP Address or domain name
              port: 8082                        # upstream port
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8082                      # health_check port
                start_available: true           # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
            - name: t2ug1u2                     # upstream name
              host: lobby-test.ipbuff.com.fail  # upstream host. IP Address or domain name
              port: 8081                        # upstream port
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8081                      # health_check port
                start_available: false          # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
      - name: target3                           # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8082                              # target port
        upstream_group:                         # upstream_group to be used for target
          name: ug2                             # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: t3ug2u1                     # upstream name
              host: lobby-test.ipbuff.com       # upstream host. IP Address or domain name
              port: 8082                        # upstream port
              dns:                              # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                        # dns address list. Queries will be done sequentially in case of failure
                  - 1.1.1.1                     # cloudflare IPv4 DNS
                  - 2606:4700::1111             # cloudflare IPv6 DNS
                ttl: 5                          # custom ttl can be specified to overwrite the DNS response TTL
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8082                      # health_check port
                start_available: true           # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
            - name: t3ug2u2                     # upstream name
              host: lobby-test.ipbuff.com       # upstream host. IP Address or domain name
              port: 8083                        # upstream port
              dns:                              # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                        # dns address list. Queries will be done sequentially in case of failure
                  - 8.8.8.8                     # google IPv4 DNS
                  - 1.1.1.1                     # cloudflare IPv4 DNS
                  - 2606:4700::1111             # cloudflare IPv6 DNS
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8083                      # health_check port
                start_available: false          # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 10            # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
`

func TestGetDistMode(t *testing.T) {
	testCases := []struct {
		input  string
		err    error
		result distMode
	}{
		{input: "round-robin", err: nil, result: distModeRR},
		{input: "weighted", err: nil, result: distModeWeighted},
		{input: "blah", err: errDistMode, result: distModeUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			dm, err := getDistMode(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.err, err)
			}
			if dm != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.result, dm)
			}
		})
	}
}

func TestDistModeString(t *testing.T) {
	testCases := []struct {
		input  distMode
		result string
	}{
		{input: distModeRR, result: "round-robin"},
		{input: distModeWeighted, result: "weighted"},
		{input: distModeUnknown, result: "unknown"},
		{input: 9, result: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.result, func(t *testing.T) {
			r := tc.input.String()
			if r != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.result, tc.result, r)
			}
		})
	}
}

func TestGetLbEngineType(t *testing.T) {
	testCases := []struct {
		input  string
		err    error
		result lbEngineType
	}{
		{input: "testEngine", err: nil, result: lbEngineTest},
		{input: "nftables", err: nil, result: lbEngineNft},
		{input: "blah", err: errLbEngineType, result: lbEngineUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := getLbEngineType(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.err, err)
			}
			if r != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.result, r)
			}
		})
	}
}

func TestNewLbEngine(t *testing.T) {
	testCases := []struct {
		input  lbEngineType
		err    error
		result lbEngine
	}{
		{input: lbEngineNft, err: nil, result: &nft{}},
		{input: lbEngineUnknown, err: errLbEngineType, result: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.input.String(), func(t *testing.T) {
			r, err := newLbEngine(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input.String(), tc.err, err)
			}
			if reflect.TypeOf(r) != reflect.TypeOf(tc.result) {
				t.Errorf(
					"%s: expected '%v', but got '%v'",
					tc.input.String(),
					reflect.TypeOf(tc.result),
					reflect.TypeOf(r),
				)
			}
		})
	}
}

func TestLbEngineTypeString(t *testing.T) {
	testCases := []struct {
		input  lbEngineType
		result string
	}{
		{input: lbEngineNft, result: "nftables"},
		{input: lbEngineUnknown, result: "unknown"},
		{input: 9, result: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.result, func(t *testing.T) {
			r := tc.input.String()
			if r != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.result, tc.result, r)
			}
		})
	}
}

func TestGetLbProtocol(t *testing.T) {
	testCases := []struct {
		input  string
		err    error
		result lbProto
	}{
		{input: "tcp", err: nil, result: lbProtoTcp},
		{input: "udp", err: nil, result: lbProtoUdp},
		{input: "sctp", err: nil, result: lbProtoSctp},
		{input: "http", err: nil, result: lbProtoHttp},
		{input: "blah", err: errLbProto, result: lbProtoUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := getLbProtocol(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.err, err)
			}
			if r != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.input, tc.result, r)
			}
		})
	}
}

func TestLbProtoString(t *testing.T) {
	testCases := []struct {
		input  lbProto
		result string
	}{
		{input: lbProtoTcp, result: "tcp"},
		{input: lbProtoUdp, result: "udp"},
		{input: lbProtoSctp, result: "sctp"},
		{input: 9, result: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.result, func(t *testing.T) {
			r := tc.input.String()
			if r != tc.result {
				t.Errorf("%s: expected '%v', but got '%v'", tc.result, tc.result, r)
			}
		})
	}
}

func TestAddAndReplaceUpstreamIps(t *testing.T) {
	ips := []string{"1.1.1.1", "2.2.2.2"}
	netIps := &[]net.IP{}
	l := &lb{}
	l.upstreamIps = &[]net.IP{}

	for _, i := range ips {
		ip := net.ParseIP(i)
		l.addUpstreamIps(ip)
		*netIps = append(*netIps, ip)
	}

	if !reflect.DeepEqual(netIps, l.upstreamIps) {
		t.Errorf("expected '%v', but got '%v'", netIps, l.upstreamIps)
	}

	l.addUpstreamIps(nil)

	rip := net.ParseIP("3.3.3.3")
	l.replaceUpstreamIps(&(*netIps)[0], &rip)
	(*netIps)[0] = rip

	if !reflect.DeepEqual(netIps, l.upstreamIps) {
		t.Errorf("expected '%v', but got '%v'", netIps, l.upstreamIps)
	}

	uip := net.ParseIP("4.4.4.4")
	err := l.replaceUpstreamIps(&uip, &rip)
	if err == nil {
		t.Errorf("expected error, but didn't get it")
	} else {
		if !errors.Is(err, errReplUpstreamIp) {
			t.Errorf("expected '%v', but got '%v'", errReplUpstreamIp, err)
		}
	}
}

func TestCheckConfig(t *testing.T) {
	config := testConfigYaml
	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	// confirm checkConfig fails on wrong engine configuration
	expectedErr := errLbEngineType
	configYaml.LbConfig[0].Engine = "blah"
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on repeated engine configuration
	wrongConfigSlice := strings.Split(config, "\n")
	wrongConfig := config + strings.Join(wrongConfigSlice[1:], "\n")
	expectedErr = errConfRepEngine
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on repeated target name
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(wrongConfig, "target2", "target1")
	expectedErr = errConfRepTargetName
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on repeated target protocol and port
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"port: 8081                              # target port",
		"port: 8080                              # target port",
	)
	expectedErr = errConfRepPortProto
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on repeated upstreamGroup name
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"ug1",
		"ug0",
	)
	expectedErr = errConfRepUGName
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on invalid target protocol
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"        protocol: tcp",
		"        protocol: blah",
	)
	expectedErr = errLbProto
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on unsupported target protocol
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"        protocol: tcp",
		"        protocol: http",
	)
	expectedErr = errConfTargetProto
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on invalid healtcheck protocol
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"                protocol: tcp                   # health-heck protocol. only tcp supported for now",
		"                protocol: blah",
	)
	expectedErr = errConfHcProtocol
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on wrong distribution mode
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"          distribution: round-robin",
		"          distribution: bleh",
	)
	expectedErr = errConfDistMode
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on repeated upstream name
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"t1ug0u2",
		"t1ug0u1",
	)
	expectedErr = errConfRepUName
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on invalid upstream host
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"8.8.8.8",
		"8.8.8.8.8",
	)
	expectedErr = errConfUHost
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}

	// confirm checkConfig fails on invalid upstream dns address
	wrongConfig = config
	wrongConfig = strings.ReplaceAll(
		wrongConfig,
		"1.1.1.1",
		"1.1.1.1.1",
	)
	expectedErr = errConfDnsAddr
	if err := yaml.Unmarshal([]byte(wrongConfig), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}
	if err := checkConfig(&configYaml); !(errors.Is(err, errLbCheckConf) &&
		errors.Is(err, expectedErr)) {
		t.Errorf(
			"checkConfig should have errored with '%v: %v', but errored with '%v'",
			errLbCheckConf,
			expectedErr,
			err,
		)
	}
}

func TestGetConfig(t *testing.T) {
	config := testConfigYaml
	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	lbc := configYaml.LbConfig[0]

	// confirm getConfig succeeds with resolved fqdn
	l := &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}

	// confirm getConfig succeeds with unresolved fqdn and dns ttl configured
	bkpHost := lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host = "unresolved.dns"
	l = &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host = bkpHost

	// confirm getConfig succeeds with unresolved fqdn and dns ttl configured
	bkpHost = lbc.TargetsConfig[2].UpstreamGroup.Upstreams[1].Host
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[1].Host = "unresolved.dns"
	l = &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[1].Host = bkpHost

	// confirm getConfig succeeds with invalid host
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host = "1.1.1.1.1"
	l = &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host = "1.1.1.1.1"
	lbc.TargetsConfig[2].UpstreamGroup.Upstreams[0].Host = bkpHost

	// fail on engine type
	bkpEngine := lbc.Engine
	lbc.Engine = "blah"
	l = &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err == nil {
		t.Errorf("expected an error to occur, but got no error")
	} else {
		if !errors.Is(err, errLbEngineType) {
			t.Errorf("expected error '%v', but got '%v'", errLbEngineType, err)
		}
	}
	lbc.Engine = bkpEngine

	// fail on distribution mode
	bkpDM := lbc.TargetsConfig[0].UpstreamGroup.Distribution
	lbc.TargetsConfig[0].UpstreamGroup.Distribution = "blah"
	l = &lb{}
	l.upstreamIps = &[]net.IP{}
	if err := l.getConfig(&lbc); err == nil {
		t.Errorf("expected an error to occur, but got no error")
	} else {
		if !errors.Is(err, errDistMode) {
			t.Errorf("expected error '%v', but got '%v'", errDistMode, err)
		}
	}
	lbc.TargetsConfig[0].UpstreamGroup.Distribution = bkpDM
}

func TestLbSCompare(t *testing.T) {
	olb := &lb{
		et: lbEngineNft,
	}
	nlb := &lb{
		et: lbEngineUnknown,
	}
	olbs := []*lb{
		olb,
	}
	nlbs := []*lb{
		olb,
		nlb,
	}

	expected := struct {
		kept  *map[lbEngineType]*lb
		niuew []*lb
		old   []*lb
	}{}

	kept := make(map[lbEngineType]*lb)
	kept[lbEngineNft] = olb
	expected.kept = &kept

	k, a, _ := lbsCompare(olbs, nlbs)

	// check kept
	kr := (*k)[lbEngineNft]
	if kr[0].et != (*expected.kept)[lbEngineNft].et {
		t.Errorf(
			"outcome didn't match for the kept lbs. expected: '%v'\nbut got '%v'",
			kr[0],
			(*expected.kept)[lbEngineNft],
		)
	}

	// check added
	if a[0].et != nlb.et {
		t.Errorf(
			"outcome didn't match for the added lbs. expected: '%v'\nbut got '%v'",
			nlb,
			a[0],
		)
	}

	_, _, r := lbsCompare(nlbs, olbs)

	// check removed
	if r[0].et != nlb.et {
		t.Errorf(
			"outcome didn't match for the removed lbs. expected: '%v'\nbut got '%v'",
			nlb,
			r[0],
		)
	}
}

func TestChecks(t *testing.T) {
	config := testConfigYaml
	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	lbc := configYaml.LbConfig[0]

	l := &lb{}
	l.upstreamIps = &[]net.IP{}
	l.state.wg = &sync.WaitGroup{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}

	l.startChecks()
	l.stopChecks()
}

func TestCheckStop(t *testing.T) {
	config := testConfigYaml
	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	lbc := configYaml.LbConfig[0]

	l := &lb{}
	l.upstreamIps = &[]net.IP{}
	l.state.wg = &sync.WaitGroup{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}

	l.startChecks()
	l.stopDcs()
	l.stopHcs()
	l.state.wg.Wait()
}

func TestInitDnsCheck(t *testing.T) {
	t.Log("Testing Init DNS Check")
	config := `lb: 
  - engine: testEngine                          # load balancer engine. only 'nftables' supported for now
    targets: 
      - name: TestInitDnsCheck                  # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8082                              # target port
        upstream_group:                         # upstream_group to be used for target
          name: ug                              # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: lt8082                      # upstream name
              host: lobby-test.ipbuff.com       # upstream host. IP Address or domain name
              port: 8082                        # upstream port
              dns:                              # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                        # dns address list. Queries will be done sequentially in case of failure
                  - 1.1.1.1                     # cloudflare IPv4 DNS
                  - 2606:4700::1111             # cloudflare IPv6 DNS
                ttl: 1                          # custom ttl can be specified to overwrite the DNS response TTL
`

	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	lbc := configYaml.LbConfig[0]

	l := &lb{}
	l.upstreamIps = &[]net.IP{}
	l.state.wg = &sync.WaitGroup{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}

	var err error
	if l.e, err = newLbEngine(l.et); err != nil {
		t.Error("%w: %w", errLbInit, err)
	}

	l.initDnsCheck(l.targets[0].upstreamGroup.upstreams[0])

	wt := 2
	t.Logf("waiting %ds for DNS check to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	// test terminate state
	l.state.m.Lock()
	l.state.t = true
	l.state.m.Unlock()
	time.Sleep(time.Duration(wt) * time.Second)
	l.stopChecks()
}

func TestInitHCheck(t *testing.T) {
	t.Log("Testing Init Health Check")
	config := `lb: 
  - engine: testEngine                          # load balancer engine. only 'nftables' supported for now
    targets: 
      - name: TestInitHealthCheck               # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8082                              # target port
        upstream_group:                         # upstream_group to be used for target
          name: ug                              # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: lt8082                      # upstream name
              host: lobby-test.ipbuff.com       # upstream host. IP Address or domain name
              port: 8082                        # upstream port
              dns:                              # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                        # dns address list. Queries will be done sequentially in case of failure
                  - 1.1.1.1                     # cloudflare IPv4 DNS
                  - 2606:4700::1111             # cloudflare IPv6 DNS
                ttl: 1                          # custom ttl can be specified to overwrite the DNS response TTL
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8082                      # health_check port
                start_available: true           # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 1             # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
`

	configYaml := ConfigYaml{}

	if err := yaml.Unmarshal([]byte(config), &configYaml); err != nil {
		t.Error("errored on config yaml parsing", err)
	}

	// confirm checkConfig succeeds
	if err := checkConfig(&configYaml); err != nil {
		t.Error("checkConfig errored unexpectedly", err)
	}

	lbc := configYaml.LbConfig[0]

	l := &lb{}
	l.upstreamIps = &[]net.IP{}
	l.state.wg = &sync.WaitGroup{}
	if err := l.getConfig(&lbc); err != nil {
		t.Errorf("getConfig errored unexpectedly: '%v'", err)
	}

	var err error
	if l.e, err = newLbEngine(l.et); err != nil {
		t.Error("%w: %w", errLbInit, err)
	}

	l.initHealthCheck(l.targets[0].upstreamGroup.upstreams[0], l.targets[0])

	wt := 3
	t.Logf("waiting %ds for Healthcheck to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	// test terminate state
	l.state.m.Lock()
	l.state.t = true
	l.state.m.Unlock()
	time.Sleep(time.Duration(wt) * time.Second)
	l.stopChecks()
}

func TestLb(t *testing.T) {
	config := `lb: 
  - engine: testEngine                          # load balancer engine. only 'nftables' supported for now
    targets: 
      - name: TestInitDnsCheck                  # target name
        protocol: tcp                           # transport protocol. only tcp supported for now
        port: 8082                              # target port
        upstream_group:                         # upstream_group to be used for target
          name: ug                              # upstream_group name
          distribution: round-robin             # ug traffic distribution mode. only round-robin supported for now
          upstreams:
            - name: lt8082                      # upstream name
              host: lobby-test.ipbuff.com       # upstream host. IP Address or domain name
              port: 8082                        # upstream port
              dns:                              # include in case you want to use specific DNS to resolve the fqdn host address. If host is IPv4 or IPv6 this setting will not have any effect. In case this mapping is not present the OS resolvers will be used
                servers:                        # dns address list. Queries will be done sequentially in case of failure
                  - 1.1.1.1                     # cloudflare IPv4 DNS
                  - 2606:4700::1111             # cloudflare IPv6 DNS
                ttl: 1                          # custom ttl can be specified to overwrite the DNS response TTL
              health_check:                     # don't include the health_check mapping or leave it empty to disable health_check. upstreams will be considered always as active when health_checks are not enabled
                protocol: tcp                   # health-heck protocol. only tcp supported for now
                port: 8082                      # health_check port
                start_available: true           # set 'true' if upstream should be considered as available at start. set 'false' otherwise
                probe:
                  check_interval: 1             # seconds. Max value: 65536
                  timeout: 2                    # seconds. Max value: 256
                  success_count: 3              # amount of successful health checks to become active
`

	// test LB init fail due to config file not found
	failedTestConfigPath := "/tmp/lobby_test_nil"
	testConfigPath := "/tmp/lobby_test_conf.yaml"

	lobbySettings.configFilePath = failedTestConfigPath

	_, err := lbInit()
	if err == nil {
		t.Errorf("lbInit should have errored due to wrong config file path")
	} else {
		if !errors.Is(err, errConfFileOpen) {
			t.Errorf("expected error '%v', but got '%v'", errConfFileOpen, err)
		}
	}

	lobbySettings.configFilePath = testConfigPath

	// test LB init fail due to invalid yaml format
	failedConfig := config
	failedConfig = strings.Replace(failedConfig, "lb:", "lb", 1)
	os.Remove(testConfigPath)
	os.WriteFile(testConfigPath, []byte(failedConfig), 0400)

	_, err = lbInit()
	if err == nil {
		confFile, _ := os.ReadFile(testConfigPath)
		t.Log("failedConfigFile:\n", string(confFile))
		t.Errorf("lbInit should have errored due to invalid yaml")
	} else {
		if !errors.Is(err, errConfFileUnmarshal) {
			t.Errorf("expected error '%v', but got '%v'", errConfFileUnmarshal, err)
		}
	}
	os.Remove(testConfigPath)

	// test LB init fail due to unsupported engine
	failedConfig = config
	failedConfig = strings.Replace(failedConfig, "engine: testEngine", "engine: blah", 1)
	os.WriteFile(testConfigPath, []byte(failedConfig), 0400)

	_, err = lbInit()
	if err == nil {
		t.Errorf("lbInit should have errored due to wrong configuration")
	} else {
		if !errors.Is(err, errLbInit) {
			t.Errorf("expected error '%v', but got '%v'", errLbInit, err)
		}
	}
	os.Remove(testConfigPath)

	// test LB init fail due to misconfigured probe: timeout
	failedConfig = config
	failedConfig = strings.Replace(failedConfig, "timeout: 2", "timeout: 0", 1)
	os.WriteFile(testConfigPath, []byte(failedConfig), 0400)

	_, err = lbInit()
	if err == nil {
		t.Errorf("lbInit should have errored due to wrong configuration")
	} else {
		if !errors.Is(err, errLbInit) {
			t.Errorf("expected error '%v', but got '%v'", errLbInit, err)
		}
	}
	os.Remove(testConfigPath)

	// test LB init fail due to misconfigured probe: check_interval
	failedConfig = config
	failedConfig = strings.Replace(failedConfig, "check_interval: 1", "check_interval: 0", 1)
	os.WriteFile(testConfigPath, []byte(failedConfig), 0400)

	_, err = lbInit()
	if err == nil {
		t.Log("Config: \n", failedConfig)
		t.Errorf("lbInit should have errored due to wrong configuration")
	} else {
		if !errors.Is(err, errLbInit) {
			t.Errorf("expected error '%v', but got '%v'", errLbInit, err)
		}
	}
	os.Remove(testConfigPath)

	// test LB init fail due to misconfigured probe: success_count
	failedConfig = config
	failedConfig = strings.Replace(failedConfig, "success_count: 3", "success_count: 0", 1)
	os.WriteFile(testConfigPath, []byte(failedConfig), 0400)

	_, err = lbInit()
	if err == nil {
		t.Errorf("lbInit should have errored due to wrong configuration")
	} else {
		if !errors.Is(err, errLbInit) {
			t.Errorf("expected error '%v', but got '%v'", errLbInit, err)
		}
	}
	os.Remove(testConfigPath)

	// test LB start fail scenarios
	os.WriteFile(testConfigPath, []byte(config), 0400)
	lbs, err := lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	for _, l := range lbs {
		e := l.e.(*testLb)

		e.setResults(false, true, false)
		if err := l.start(); err == nil {
			t.Errorf("lb start should have errored due to failed permissions")
		} else {
			if !errors.Is(err, errCheckPerm) {
				t.Errorf("expected error '%v', but got '%v'", errCheckPerm, err)
			}
		}

		e.setResults(true, false, false)
		if err := l.start(); err == nil {
			t.Errorf("lb start should have errored due to failed dependencies")
		} else {
			if !errors.Is(err, errCheckDep) {
				t.Errorf("expected error '%v', but got '%v'", errCheckDep, err)
			}
		}

		e.setResults(false, false, true)
		if err := l.start(); err == nil {
			t.Errorf("lb start should have errored due to failed start")
		} else {
			if !errors.Is(err, errLbEngineStart) {
				t.Errorf("expected error '%v', but got '%v'", errLbEngineStart, err)
			}
		}
	}
	os.Remove(testConfigPath)

	// test successful LB start
	os.WriteFile(testConfigPath, []byte(config), 0400)
	lbs, err = lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}

	for _, l := range lbs {
		if err := l.start(); err != nil {
			t.Errorf("lb start returned an unexpected error: '%v'", err)
		}
		if err := lbReconfig(&lbs); err != nil {
			t.Errorf("lbReconfig returned an unexpected error: '%v'", err)
		}
	}
	os.Remove(testConfigPath)

	// test failed lb update due to invalid yaml
	updatedConfig := strings.Replace(config, "host: lobby-test.ipbuff.com", "host lobby-test.ipbuff.comb", 1)
	os.WriteFile(testConfigPath, []byte(updatedConfig), 0400)
	if err := lbReconfig(&lbs); err == nil {
		t.Errorf("lb start should have errored due to failed start")
	} else {
		if !errors.Is(err, errLbInit) {
			t.Errorf("expected error '%v', but got '%v'", errLbInit, err)
		}
	}
	os.Remove(testConfigPath)

	// test failed lb update due to invalid engine
	updatedConfig = strings.Replace(config, "engine: testEngine", "engine: blah", 1)
	os.WriteFile(testConfigPath, []byte(updatedConfig), 0400)
	if err := lbReconfig(&lbs); err == nil {
		t.Errorf("lb start should have errored due to failed start")
	} else {
		if !errors.Is(err, errLbEngineType) {
			t.Errorf("expected error '%v', but got '%v'", errLbEngineType, err)
		}
	}
	os.Remove(testConfigPath)

	// test update config by removing an lb engine
	updatedConfig = `lb: 
`
	os.WriteFile(testConfigPath, []byte(updatedConfig), 0400)
	if err := lbReconfig(&lbs); err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	os.Remove(testConfigPath)

	// test update config by adding an lb engine
	os.WriteFile(testConfigPath, []byte(config), 0400)
	if err := lbReconfig(&lbs); err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	os.Remove(testConfigPath)

	lbs[0].stop()
}
