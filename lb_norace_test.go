//go:build !race
// +build !race

package main

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// can't test with -race flag due to defaultDnsResolver change

func TestLbChanges(t *testing.T) {
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

	testConfigPath := "/tmp/lobby_test_conf.yaml"
	lobbySettings.configFilePath = testConfigPath
	os.WriteFile(testConfigPath, []byte(config), 0400)

	// mock dns resolvers
	var mockResolverSucceed resolver = func(f string, dnsa []string) (net.IP, uint32, error) {
		return net.ParseIP("1.1.1.1"), 600, nil
	}
	var mockResolverFail resolver = func(f string, dnsa []string) (net.IP, uint32, error) {
		return net.ParseIP("1.1.1.1"), 600, fmt.Errorf("Some DNS error")
	}

	wt := 6

	// start by testing dns check fail on boot
	defaultDnsResolver = mockResolverFail

	lbs, err := lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	for _, l := range lbs {
		if err := l.start(); err != nil {
			t.Errorf("lb start returned an unexpected error: '%v'", err)
		}
	}
	os.Remove(testConfigPath)

	t.Logf("waiting %ds for LB to boot", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	// test success
	defaultDnsResolver = mockResolverSucceed
	t.Logf("waiting %ds for DNS check and upstream update to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)
	defaultDnsResolver = miekgResolver
	t.Logf("waiting %ds for DNS check and upstream update to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)
	// test failure
	defaultDnsResolver = mockResolverFail
	t.Logf("waiting %ds for DNS check and upstream update to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	defaultDnsResolver = miekgResolver
	lbs[0].stop()
}

func Test0TTL(t *testing.T) {
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
`

	testConfigPath := "/tmp/lobby_test_conf.yaml"
	lobbySettings.configFilePath = testConfigPath
	os.WriteFile(testConfigPath, []byte(config), 0400)

	// mock dns resolvers
	var mockResolverSucceed resolver = func(f string, dnsa []string) (net.IP, uint32, error) {
		return net.ParseIP("1.1.1.1"), 1, nil
	}
	var mockResolver0TTL resolver = func(f string, dnsa []string) (net.IP, uint32, error) {
		return net.ParseIP("1.1.1.1"), 0, nil
	}

	wt := 3

	defaultDnsResolver = mockResolverSucceed

	lbs, err := lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	for _, l := range lbs {
		if err := l.start(); err != nil {
			t.Errorf("lb start returned an unexpected error: '%v'", err)
		}
	}

	t.Logf("waiting %ds for LB to boot", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	// test returned 0 TTL
	defaultDnsResolver = mockResolver0TTL
	t.Logf("waiting %ds for DNS check and upstream update to complete", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	lbs[0].stop()

	// Now test with 0 TTL at start
	defaultDnsResolver = mockResolver0TTL

	lbs, err = lbInit()
	if err != nil {
		t.Errorf("lbInit returned an unexpected error: '%v'", err)
	}
	for _, l := range lbs {
		if err := l.start(); err != nil {
			t.Errorf("lb start returned an unexpected error: '%v'", err)
		}
	}
	os.Remove(testConfigPath)

	t.Logf("waiting %ds for LB to boot", wt)
	time.Sleep(time.Duration(wt) * time.Second)

	lbs[0].stop()
	defaultDnsResolver = miekgResolver
}
