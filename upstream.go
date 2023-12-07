package main

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/nftables"
)

type (
	hcProto  byte // healtcheck protocol
	ugFoMode byte // upstream group failover mode
)

const (
	hcProtoUnknown hcProto = iota // undefined
	hcProtoTcp                    // tcp
	hcProtoUdp                    // udp
	hcProtoSctp                   // sctp
	hcProtoHttp                   // http
	hcProtoGrpc                   // grpc
)

const numUgFoModes = 5 // amount of ugFoMode's

const (
	ugFoModeUnknown ugFoMode = iota
	ugFoModeInactive
	ugFoModeActive1
	ugFoModeActive2
	ugFoModeDown
)

// upstream errors
var (
	errHcp = errors.New(
		"healtcheck protocol not found",
	)
	errUgFM = errors.New(
		"error providing next upstream failover mode",
	)
)

func getHcProto(hcp string) (hcProto, error) {
	switch hcp {
	case "tcp":
		return hcProtoTcp, nil
	case "udp":
		return hcProtoUdp, nil
	case "sctp":
		return hcProtoSctp, nil
	case "http":
		return hcProtoHttp, nil
	case "grpc":
		return hcProtoGrpc, nil
	}

	return hcProtoUnknown, fmt.Errorf("'%s' '%w'", hcp, errHcp)
}

func (hcp hcProto) String() string {
	switch hcp {
	case hcProtoTcp:
		return "tcp"
	case hcProtoUdp:
		return "udp"
	case hcProtoSctp:
		return "sctp"
	case hcProtoHttp:
		return "http"
	case hcProtoGrpc:
		return "grpc"
	}
	return "unknown"
}

type upstreamDns struct {
	addresses []string      // DNS addresses to be used to resolve the upstream host domain name
	confTtl   uint32        // user configured DNS TTL to overwrite DNS resolved TTL
	ttl       uint32        // DNS TTL to be used. confTtl will be used if set. Otherwise, the DNS resolved TTL
	chDcStop  chan struct{} // channel to listen to upstream dns stop requests
	ticker    *time.Ticker  // DNS check timer. Always set to upstreamDns.ttl
}

type healthCheck struct {
	active        bool          // healthcheck active or inactive
	protocol      hcProto       // healthcheck protocol
	port          uint16        // healtcheck port
	checkInterval uint16        // healtcheck check interval in seconds
	timeout       uint8         // healthcheck check timeout
	countConfig   uint8         // healthcheck configured consecutive successful checks required to become available
	count         uint8         // healthcheck variable used to count progress of consecutive successful checks
	chHcStop      chan struct{} // channel to listen to healthcheck stop requests
	ticker        *time.Ticker  // healtcheck timer
}

// An upstream is a host where the traffic can be distributed to
type upstream struct {
	name        string      // upstream name
	protocol    lbProto     // upstream layer 4 protocol
	host        string      // upstream host. It can be an IP address or a domain name
	port        uint16      // upstream port
	dns         upstreamDns // upstream DNS. used to resolve upstream host if a domain name
	address     net.IP      // upstream IP address. It is either the IP address from upstream host or the resolved upstream host domain name
	available   bool        // upstream state. available or unavailable
	healthCheck healthCheck // upstream healtcheck configuration
}

// returns the ugFoMode ID
func (ugFM ugFoMode) getId() string {
	switch ugFM {
	case ugFoModeUnknown:
		return "0"
	case ugFoModeInactive:
		return "1"
	case ugFoModeActive1:
		return "2"
	case ugFoModeActive2:
		return "3"
	case ugFoModeDown:
		return "4"
	}

	return "0"
}

// returns the next ugFoMode
func (ugFM ugFoMode) nextMode() (ugFoMode, error) {
	switch ugFM {
	case ugFoModeInactive:
		return ugFoModeActive1, nil
	case ugFoModeActive1:
		return ugFoModeActive2, nil
	case ugFoModeActive2:
		return ugFoModeActive1, nil
	case ugFoModeDown:
		return ugFoModeActive1, nil
	}

	return ugFoModeUnknown, fmt.Errorf("%w", errUgFM)
}

// An upstream group is a group of upstreams serving the same target
type upstreamGroup struct {
	name                 string
	distMode             distMode
	upstreams            []*upstream
	failoverMode         ugFoMode
	previousFailoverMode ugFoMode
	nftUgChain           []*nftables.Chain
	nftUgSet             []*nftables.Set
	nftUgChainRule       []*nftables.Rule
	nftCounter           *nftables.CounterObj
}
