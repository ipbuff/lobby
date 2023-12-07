package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Hardcoded settings
var (
	// Currently supported healtcheck protocol
	supHcProto = map[hcProto]bool{
		hcProtoTcp: true, // TCP
	}
)

// Lobby doesn't implements the traffic load balancing. It orchestrates load balancer engines (lbe) for traffic load balancing
// An lbe is the runtime load balancer which performs the traffic load balancing
// More than one lbe can be operating simultaneously
// Each lbe has to implement the functions defined by the 'lbEngine' interface
// Which are required by Lobby for load balancing orchestration
type lbEngine interface {
	checkDependencies() error                       // checks if the lb engine dependencies are satisfied
	checkPermissions() error                        // checks if the lb runtime permissions are sufficient for the load balancer engine to function successfully
	getCapabilities() map[lbProto]map[distMode]bool // returns lb capabilities. key protocols, value distribution mode
	start(*lb) error                                // starts lb
	stop() error                                    // stops lb
	reconfig(*lb) error                             // reconfigures lb
	updateTarget(*target) error                     // updates given target
	updateUpstream(*upstream, *[]net.IP) error      // updates given upstream
}

// Load balancer state
type lbState struct {
	m  sync.Mutex      // load balancer changes mutex
	wg *sync.WaitGroup // wait group to keep track of load balancer go routines
	t  bool            // request to terminate
}

// Load balancer
type lb struct {
	targets     []*target    // list of all load balancer targets
	e           lbEngine     // load balancer engine
	et          lbEngineType // load balancer engine type
	upstreamIps *[]net.IP    // list of all upstream IP addresses
	state       lbState      // load balancer state
}

// iota constants
type (
	distMode     byte  // distribution mode
	lbEngineType uint8 // load balancer engine type
	lbProto      uint8 // load balancer protocol
)

// distribution mode
const (
	distModeUnknown  distMode = iota // undefined
	distModeRR                       // round robin
	distModeWeighted                 // weighted
)

const (
	lbEngineUnknown lbEngineType = iota // undefined
	lbEngineTest                        // test engine
	lbEngineNft                         // nftables
)

// Loadbalancer protocols
const (
	lbProtoUnknown lbProto = iota // undefined
	lbProtoTcp                    // tcp
	lbProtoUdp                    // udp
	lbProtoSctp                   // sctp
	lbProtoHttp                   // http
)

// Load Balancer errors
var (
	errCheckDep = errors.New(
		"Dependencies check failed",
	)
	errCheckPerm = errors.New(
		"Permissions check failed",
	)
	errConfFileOpen = errors.New(
		"Failed to open config file. Check if the file exists or read permissions",
	)
	errConfFileUnmarshal = errors.New(
		"Error when unmarshaling yaml config file",
	)
	errConfRepEngine = errors.New(
		"Error in configuration. Found repeated engine type. All config of a given engine type must be included in a single mapping",
	)
	errConfRepTargetName = errors.New(
		"Error in configuration. Found repeated target name. Every target name must be unique",
	)
	errConfRepUGName = errors.New(
		"Error in configuration. Found repeated upstream group name. Every upstream group name must be unique",
	)
	errConfRepPortProto = errors.New(
		"Error in configuration. Found repeated port/protocol. Each target must have a unique port/protocol pair",
	)
	errConfRepUName = errors.New(
		"Error in configuration. Found repeated upstream name. Every upstream name must be unique",
	)
	errConfUHost = errors.New(
		"Error in configuration. Found invalid host",
	)
	errConfDnsAddr = errors.New(
		"Error in configuration. Found invalid DNS address. All configured DNS addresses must be valid",
	)
	errConfDistMode = errors.New(
		"Error in configuration. Found unsupported distribution mode",
	)
	errConfTargetProto = errors.New(
		"Error in configuration. Found unsupported target protocol",
	)
	errConfHcProtocol = errors.New(
		"Error in configuration. Found unsupported upstream healthcheck protocol",
	)
	errConfProbePort = errors.New(
		"Error in configuration. Found problematic health check probe port",
	)
	errConfProbeCI = errors.New(
		"Error in configuration. Found problematic health check probe check interval",
	)
	errConfProbeSC = errors.New(
		"Error in configuration. Found problematic health check success count definition",
	)
	errConfProbeTimeout = errors.New(
		"Error in configuration. Found problematic health check timeout value",
	)
	errDistMode = errors.New(
		"distribution mode not found",
	)
	errLbEngineType = errors.New(
		"Error requesting unknown engine type",
	)
	errLbInit = errors.New(
		"Error during Load Balancer setup",
	)
	errLbEngineStart = errors.New(
		"Error during Load Balancer startup",
	)
	errLbEngineStop = errors.New(
		"Error during Load Balancer shutdown",
	)
	errLbReconfig = errors.New(
		"Error during Load Balancer reconfiguration",
	)
	errLbEngineReconfig = errors.New(
		"Error during Load Balancer engine reconfiguration",
	)
	errLbCheckConf = errors.New(
		"Error when checking Load Balancer configuration",
	)
	errLbConf = errors.New(
		"Error when loading Load Balancer configuration",
	)
	errLbProto = errors.New(
		"LB protocol not found",
	)
	errLbEngineUpstreamUpdate = errors.New(
		"Error during upstream update",
	)
	errReplUpstreamIp = errors.New(
		"Error occurred when replacing upstream IP's",
	)
)

// getDistMode returns the distMode (distribution mode) from a string
func getDistMode(dm string) (distMode, error) {
	switch dm {
	case "round-robin":
		return distModeRR, nil
	case "weighted":
		return distModeWeighted, nil
	}
	return distModeUnknown, fmt.Errorf("'%s' '%w'", dm, errDistMode)
}

// returns the string value of the distMode (distribution mode)
func (dm distMode) String() string {
	switch dm {
	case distModeRR:
		return "round-robin"
	case distModeWeighted:
		return "weighted"
	}
	return "unknown"
}

// getLbEngineType returns the lbEngineType (load balancer engine type) from a string
func getLbEngineType(lbet string) (lbEngineType, error) {
	switch lbet {
	case "testEngine":
		return lbEngineTest, nil
	case "nftables":
		return lbEngineNft, nil
	}
	return lbEngineUnknown, errLbEngineType
}

// newLbEngine returns an initialized lb engine for the requested lbEngineType
func newLbEngine(lbet lbEngineType) (lbEngine, error) {
	switch lbet {
	case lbEngineTest: // test LB
		return &testLb{}, nil
	case lbEngineNft: // nftables
		return &nft{}, nil
	}
	return nil, errLbEngineType
}

// returns the string value of the lbEngineType (load balancer engine type)
func (lbet lbEngineType) String() string {
	switch lbet {
	case lbEngineTest:
		return "testEngine"
	case lbEngineNft:
		return "nftables"
	}

	return "unknown"
}

// getLbProtocol returns the getLbProtocol (load balancer protocol) from a string
func getLbProtocol(lbp string) (lbProto, error) {
	switch lbp {
	case "tcp":
		return lbProtoTcp, nil
	case "udp":
		return lbProtoUdp, nil
	case "sctp":
		return lbProtoSctp, nil
	case "http":
		return lbProtoHttp, nil
	}

	return lbProtoUnknown, fmt.Errorf("'%s' '%w'", lbp, errLbProto)
}

// returns the string value of the lbProto (load balancer protocol)
func (lbp lbProto) String() string {
	switch lbp {
	case lbProtoTcp:
		return "tcp"
	case lbProtoUdp:
		return "udp"
	case lbProtoSctp:
		return "sctp"
	}
	return "unknown"
}

// addUpstreamIps adds the provided IP address to the lb.upstreamIps slice
// holding all load blancer upstream IP addresses
func (l *lb) addUpstreamIps(nip net.IP) {
	if nip == nil {
		return
	}

	*l.upstreamIps = append(*l.upstreamIps, nip)

	return
}

// replaceUpstreamIps replaces an IP in the lb.upstreamIps slice
// 'oip' is the IP to be removed and 'nip' is the new IP to take its place
// As this slice should have all IP's from all upstreams for the load balancer,
// the IP to be removed is expected to exist
// In case the 'oip' is not found the error errReplUpstreamIp is returned
func (l *lb) replaceUpstreamIps(oip *net.IP, nip *net.IP) error {
	uip := *l.upstreamIps
	for i, ip := range uip {
		if ip.Equal(*oip) {
			uip[i] = *nip
			return nil
		}
	}

	return fmt.Errorf(
		"%w: Failed to replace '%s' with '%s' in upstream IP's list. '%s' not found",
		errReplUpstreamIp,
		oip.String(),
		nip.String(),
		oip.String(),
	)
}

// checkConfig checks the configuration file
// It verifies that:
//   - only supported engine types are configured
//   - all engine types, target, upstream group and upstream names are unique
//   - targets do not have conflicting port/protocol configuration
//   - target protocols are supported by the engine
//   - the configured distribution mode is supported as defined in global var supDM
//   - the host format is valid
//   - upstream healtcheck protocols are supported
//   - DNS addresses are valid
//
// It returns a errLbCheckConf error if check fails
func checkConfig(configYaml *ConfigYaml) error {
	LogDVf("LB: configuration check")
	var (
		eNames  []string
		tNames  []string
		uNames  []string
		ugNames []string
	)

	for i, lbc := range configYaml.LbConfig {
		// Check load balancer engine
		lbE, err := getLbEngineType(lbc.Engine)
		if err != nil {
			return fmt.Errorf("%w: %w", errLbCheckConf, err)
		}
		lben := lbE.String()

		LogDVf("LB: checking '%s' load balancer engine uniqueness", lben)
		if i == 0 {
			// Initialize temporary var
			eNames = append(eNames, lben)
		} else {
			for _, en := range eNames {
				if en == lben {
					LogDf("LB: found a repeated engine type: %s", lbc.Engine)
					return fmt.Errorf("%w: %w: problematic engine in config: %s", errLbCheckConf, errConfRepEngine, lbc.Engine)
				}
			}
		}

		e, _ := newLbEngine(lbE)
		ec := e.getCapabilities()

		// Create a map with the port number as the key and slice of strings for the protocol value
		portMap := make(map[uint16][]string)

		for i, t := range lbc.TargetsConfig {
			LogDVf("LB: target '%s' check", t.Name)
			if i == 0 {
				// Initialize temporary vars
				tNames = append(tNames, t.Name)
				portMap[t.Port] = append(portMap[t.Port], t.Protocol)
				ugNames = append(ugNames, t.UpstreamGroup.Name)
			} else {
				// Check if target names are unique
				for _, tn := range tNames {
					if tn == t.Name {
						LogDf("LB: found a repeated target name: %s", tn)
						return fmt.Errorf("%w: %w: problematic target name in config: %s", errLbCheckConf, errConfRepTargetName, tn)
					}
				}
				tNames = append(tNames, t.Name)

				// Check if target protocol and port are unique
				if protos, ok := portMap[t.Port]; ok {
					LogDVf("Value: %v", protos)
					for _, proto := range protos {
						if proto == t.Protocol {
							LogDf("Found a repeated target port/protocol configuration: %d/%s", t.Port, t.Protocol)
							return fmt.Errorf("%w: %w: Problematic port/protocol: %d/%s", errLbCheckConf, errConfRepPortProto, t.Port, t.Protocol)
						}
					}
				} else {
					portMap[t.Port] = append(portMap[t.Port], t.Protocol)
				}

				// Check if upstreamGroup names are unique
				for _, ugn := range ugNames {
					if ugn == t.UpstreamGroup.Name {
						LogDf("LB: found a repeated upstream group name in config: %s", ugn)
						return fmt.Errorf("%w: %w: Problematic upstream group name: %s", errLbCheckConf, errConfRepUGName, ugn)
					}
				}
				ugNames = append(ugNames, t.UpstreamGroup.Name)

			}

			// Check target protocol
			tP, err := getLbProtocol(t.Protocol)
			if err != nil {
				return fmt.Errorf("%w: %w", errLbCheckConf, err)
			}
			if _, ok := ec[tP]; !ok {
				var supP []string
				for p := range ec {
					supP = append(supP, fmt.Sprintf("'%s'", p.String()))
				}
				return fmt.Errorf(
					"%w: %w: unsupported protocol '%s' for target '%s'. Chose one of the supported protocols: %s",
					errLbCheckConf,
					errConfTargetProto,
					t.Protocol,
					t.Name,
					strings.Join(supP, ", "),
				)
			}

			// Check if upstreamGroup distribution mode is supported
			dMode, _ := getDistMode(t.UpstreamGroup.Distribution)
			if _, ok := ec[tP][dMode]; !ok {
				var supDms []string
				for dm := range ec[tP] {
					supDms = append(supDms, fmt.Sprintf("'%s'", dm.String()))
				}
				return fmt.Errorf(
					"%w: %w: unsupported distribution mode: %s. Chose one of the supported modes: %s",
					errLbCheckConf,
					errConfDistMode,
					t.UpstreamGroup.Distribution,
					strings.Join(supDms, ", "),
				)
			}

			// Check upstreams
			for _, u := range t.UpstreamGroup.Upstreams {
				LogDVf("LB: upstream '%s' check", u.Name)

				// Check if upstream names are unique
				if len(uNames) == 0 {
					uNames = append(uNames, u.Name)
				} else {
					for _, un := range uNames {
						if un == u.Name {
							LogDf("Found a repeated upstream name: %s", un)
							return fmt.Errorf("%w: %w: Problematic upstream name: %s", errLbCheckConf, errConfRepUName, un)
						}
					}
					uNames = append(uNames, u.Name)
				}

				// Check upstream host
				uh, _ := getHostType(u.Host)
				if uh == hostTypeUnknown {
					return fmt.Errorf(
						"%w: %w: host '%s' for upstream '%s' is invalid. Set a valid host in the FQDN, IPv4 or IPv6 format",
						errLbCheckConf,
						errConfUHost,
						u.Host,
						u.Name,
					)
				}

				// Check upstream healthcheck:
				// - protocol
				// - port
				// - check_interval
				// - success_count
				// - timeout
				if u.HealthCheck != (HealthCheckConfig{}) {
					hcP, _ := getHcProto(u.HealthCheck.Protocol)
					if _, ok := supHcProto[hcP]; !ok {
						var supHcP []string
						for p := range supHcProto {
							supHcP = append(supHcP, fmt.Sprintf("'%s'", p.String()))
						}
						return fmt.Errorf(
							"%w: %w: unsupported upstream healthcheck protocol '%s' for upstream '%s'. Chose one of the supported protocols: %s",
							errLbCheckConf,
							errConfHcProtocol,
							u.HealthCheck.Protocol,
							u.Name,
							strings.Join(supHcP, ", "),
						)
					}

					if u.HealthCheck.Port == 0 {
						return fmt.Errorf("%w: %w: health check probe 'port' for upstream '%s' must be correctly defined",
							errLbCheckConf,
							errConfProbePort,
							u.Name)
					}

					if u.HealthCheck.Probe.CheckInterval == 0 {
						return fmt.Errorf("%w: %w: health check probe 'check_interval' for upstream '%s' must be defined",
							errLbCheckConf,
							errConfProbeCI,
							u.Name)
					}

					if u.HealthCheck.Probe.Count == 0 {
						return fmt.Errorf("%w: %w: health check probe 'success_count' for upstream '%s' must be defined",
							errLbCheckConf,
							errConfProbeSC,
							u.Name)
					}

					if u.HealthCheck.Probe.Timeout == 0 {
						return fmt.Errorf("%w: %w: health check probe 'timeout' for upstream '%s' must be defined",
							errLbCheckConf,
							errConfProbeTimeout,
							u.Name)
					}
				}

				// Check DNS addresses
				for _, a := range u.Dns.Servers {
					if net.ParseIP(a) == nil {
						LogDf("Invalid DNS address: %s", a)
						return fmt.Errorf(
							"%w: %w: Problematic DNS address: %s",
							errLbCheckConf,
							errConfDnsAddr,
							a,
						)
					}
				}
			}

		}
	}

	return nil
}

// getConfig parses the load balancer engine configuration
// It assumes that the config has been already checked for errors or mistakes
// Returns an error in case of failure
func (l *lb) getConfig(lbc *LbConfig) error {
	if et, err := getLbEngineType(lbc.Engine); err != nil {
		return fmt.Errorf("%w: %w", errLbConf, err)
	} else {
		l.et = et
	}

	// For each target
	for _, t := range lbc.TargetsConfig {
		// Distribution mode config
		dMode, err := getDistMode(t.UpstreamGroup.Distribution)
		if err != nil {
			return fmt.Errorf("%w: %w", errLbConf, err)
		}

		// Initalize Upstream Group
		ug := upstreamGroup{
			name:         t.UpstreamGroup.Name,
			distMode:     dMode,
			failoverMode: ugFoModeInactive,
		}

		// Target protocol config
		lbp, _ := getLbProtocol(t.Protocol)

		// For each upstream
		for _, u := range t.UpstreamGroup.Upstreams {
			var (
				hcActive        bool
				hcProto         hcProto
				uStartAvailable bool
			)

			// Upstream health check
			if u.HealthCheck != (HealthCheckConfig{}) {
				hcActive = true
				uStartAvailable = u.HealthCheck.StartAvailable
				hcProto, _ = getHcProto(u.HealthCheck.Protocol)
			} else {
				// Health check inactive for this upstream
				hcActive = false
				uStartAvailable = true
			}

			// Upstream IP Address
			var ipa net.IP
			var lttl uint32
			ht, _ := getHostType(u.Host)
			switch ht {
			case hostTypeFqdn:
				var err error

				// If host is a name, set it as a FQDN with trailing dot
				if !strings.HasSuffix(u.Host, ".") {
					u.Host = u.Host + "."
				}

				// Get A Record IP address for FQDN
				// If it fails to resolve, set IP Address to nil and do not start available
				// In case of failure, a new DNS query will be performed in the configured
				// ttl or in the default ttl
				ipa, lttl, err = resolveFqdn(u.Host, u.Dns.Servers, nil)
				if err != nil {
					LogWf(
						"LB: failed to resolve IP for host '%s' for upstream '%s'",
						u.Host,
						u.Name,
					)
					if u.Dns.Ttl != 0 {
						lttl = u.Dns.Ttl
					} else {
						lttl = lobbySettings.defaultDnsTtl
					}
					LogWf(
						"LB: setting upstream '%s' as unavailable. New DNS query query will be performed in '%d' seconds",
						u.Name,
						lttl,
					)
					ipa = nil
					uStartAvailable = false
				}
				LogDVf("LB: Initial FQDN upstream address '%s' and DNS TTL %ds", ipa.String(), lttl)
			case hostTypeIPv4, hostTypeIPv6:
				ipa = net.ParseIP(u.Host)
			default:
				LogWf("LB: failed to process host '%s' for upstream '%s'", u.Host, u.Name)
				LogWf("LB: setting upstream '%s' as unavailable", u.Name)
				ipa = nil
				uStartAvailable = false
			}
			LogDf("LB: upstream '%s' address: '%s'", u.Name, ipa)

			// Add upstream IP Address to load balancer list of IP addresses
			l.addUpstreamIps(ipa)

			// Upstream initialization
			newUpstream := upstream{
				name:     u.Name,
				protocol: lbp,
				host:     u.Host,
				port:     u.Port,
				dns: upstreamDns{
					addresses: u.Dns.Servers,
					confTtl:   u.Dns.Ttl,
					ttl:       lttl,
					chDcStop:  make(chan struct{}),
				},
				address:   ipa,
				available: uStartAvailable,
				healthCheck: healthCheck{
					active:        hcActive,
					protocol:      hcProto,
					port:          u.HealthCheck.Port,
					checkInterval: u.HealthCheck.Probe.CheckInterval,
					timeout:       u.HealthCheck.Probe.Timeout,
					countConfig:   u.HealthCheck.Probe.Count,
					count:         0,
					chHcStop:      make(chan struct{}),
				},
			}

			// Add upstream to upstream group
			ug.upstreams = append(ug.upstreams, &newUpstream)
		}

		// Target initialization
		newTarget := target{
			name:          t.Name,
			protocol:      lbp,
			ip:            t.Ip,
			port:          t.Port,
			upstreamGroup: &ug,
		}

		l.targets = append(l.targets, &newTarget)
	}

	return nil
}

// stopHcs stops the load balancer engine healthchecks
// The stop is triggered when the healthcheck channel is closed
func (l *lb) stopHcs() {
	for _, t := range l.targets {
		for _, u := range t.upstreamGroup.upstreams {
			close(u.healthCheck.chHcStop)
		}
	}
}

// stopDcs stops the load balancer engine DNS checks
// The stop is triggered when the DNS check channel is closed
func (l *lb) stopDcs() {
	for _, t := range l.targets {
		for _, u := range t.upstreamGroup.upstreams {
			close(u.dns.chDcStop)
		}
	}
}

// stopChecks requests the healthcheck and DNS checks to stop
// It uses waitgroups to wait until all are stopped and only then it returns
func (l *lb) stopChecks() {
	LogIf("LB: stopping health checks for '%s'", l.et.String())
	l.stopHcs()
	LogIf("LB: stopping dns checks")
	l.stopDcs()
	l.state.wg.Wait()
	LogDf("LB: health checks and dns checks stopped")
}

// startChecks initializes the DNS checks and health checks for all upstreams
func (l *lb) startChecks() {
	LogIf("LB: starting checks for '%s'", l.et.String())

	// For each target
	for _, t := range l.targets {
		// For each upstream: initalize dns and health checks
		for _, u := range t.upstreamGroup.upstreams {
			LogDVf("LB: initializing DNS checks for target '%s'", t.name)
			ht, _ := getHostType(u.host)
			if ht == hostTypeFqdn {
				u.dns.chDcStop = make(chan struct{})
				l.initDnsCheck(u)
			}
			LogDVf("LB: initializing health checks for target '%s'", t.name)
			if u.healthCheck.active {
				l.initHealthCheck(u, t)
			}
		}
	}
}

// stop is used to stop the load balancer engine
// It stops all load balancers checks and then
// requests the load balancer engine to stop
func (l *lb) stop() error {
	LogIf("LB: stop Load Balancer requested")
	l.stopChecks()
	if err := l.e.stop(); err != nil {
		return fmt.Errorf("%w: %w", errLbEngineStop, err)
	}
	LogIf("LB: load balancer successfully stopped")
	return nil
}

// start function starts the load balancer engine
// It initializes the engine, requests the engine to confirm permissions and dependencies
// Then it starts load balancing and finally the DNS and health checks are initiated
// All errors are treated as non recoverable functional impediment
func (l *lb) start() error {
	LogIf("LB: start Load Balancer requested")

	// initialize load balancer engine
	LogDf("LB: initializing load balancer engine '%s'", l.et.String())
	if err := l.e.checkPermissions(); err != nil {
		return fmt.Errorf("%w: %w: %w", errLbEngineStart, errCheckPerm, err)
	}

	if err := l.e.checkDependencies(); err != nil {
		return fmt.Errorf("%w: %w: %w", errLbEngineStart, errCheckDep, err)
	}

	if err := l.e.start(l); err != nil {
		return fmt.Errorf("%w: %w", errLbEngineStart, err)
	}

	l.startChecks()

	LogIf("LB: the load balancer was successfully started")

	return nil
}

// initHealthCheck initiates the healthcheck routines for the upstream
func (l *lb) initHealthCheck(u *upstream, t *target) {
	e := l.e
	ln := l.et.String()

	// Initialize the random number generator with a seed based on the current time
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random number between 1 and maxHcTimerInit (inclusive)
	// Intn generates 0 which we don't want, hence 1 + r.Intn
	initTicker := 1 + r.Intn(lobbySettings.maxHcTimerInit)

	// Initialize ticker with a random duration to avoid all healthchecks starting at the same millisecond
	LogDVf("LB HC (%s): upstream healthcheck timer initialized to %dms", u.name, initTicker)
	u.healthCheck.ticker = time.NewTicker(time.Duration(initTicker) * time.Millisecond)

	l.state.wg.Add(1)
	go func() {
		defer l.state.wg.Done()

		for {
			select {
			case <-u.healthCheck.chHcStop:
				LogIf("LB HC (%s): healthcheck stop requested", u.name)

				// complete go routine
				return
			case <-u.healthCheck.ticker.C:
				LogDVf("LB HC (%s): healthcheck timer trigger", u.name)

				l.state.m.Lock() // This has been included to be able to pause the healthcheck for instance in case of reconfiguration
				LogDVf("LB: '%s' load balancer engine changes are locked", ln)
				// Check if the load balancer is in 'terminate' state
				// if it is skip the healthcheck
				if l.state.t {
					l.state.m.Unlock()
					LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
					continue
				}
				LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
				l.state.m.Unlock()

				var addr string
				if u.address != nil {
					addr = u.address.String() + ":" + strconv.Itoa(int(u.healthCheck.port))
				} else {
					LogDVf("LB HC (%s): Host '%s' with unresolved address. Health check paused while host address is not available", u.name, u.host)
				}

				LogDVf(
					"LB HC (%s): healthchecking upstream IP: '%s'; Port: '%d'; Protocol: '%s'; Timeout: '%d' seconds",
					u.name,
					u.address.String(),
					u.healthCheck.port,
					u.healthCheck.protocol.String(),
					u.healthCheck.timeout,
				)
				c, err := net.DialTimeout(
					u.healthCheck.protocol.String(),
					addr,
					time.Duration(u.healthCheck.timeout)*time.Second,
				)
				if err != nil {
					LogIf(
						"LB HC (%s): healthcheck for upstream failed. Retrying in %ds. Error: %v",
						u.name,
						u.healthCheck.checkInterval,
						err,
					)

					// Reset health_check count to 0
					u.healthCheck.count = 0

					if u.available {
						// If upstream was marked as available
						u.available = false
						LogIf(
							"LB HC (%s): upstream became unavailable due to health_check failure",
							u.name,
						)
						// update nftables
						e.updateTarget(t)
					}
				} else {
					// If health_check succeeds, close net.Conn
					c.Close()

					if !u.available {
						// If upstream in not available state
						// Increment health_check count
						u.healthCheck.count++
						LogIf("LB HC (%s): upstream is unavailable at '%s', but health_check succeeded. %d/%d tests succeeded", u.name, addr, u.healthCheck.count, u.healthCheck.countConfig)
						if u.healthCheck.count >= u.healthCheck.countConfig {
							u.available = true
							LogIf("LB HC (%s): upstream became available at '%s'", u.name, addr)
							// update nftables
							e.updateTarget(t)
						}
					} else {
						LogDVf("LB HC (%s): upstream continues available at %s", u.name, addr)
					}
				}
				// Reset healthcheck timer
				LogDVf(
					"LB HC (%s): healthcheck recheck in %ds",
					u.name,
					u.healthCheck.checkInterval,
				)
				u.healthCheck.ticker.Reset(time.Duration(u.healthCheck.checkInterval) * time.Second)
			}
		}
	}()
}

// initDnsCheck initiates the DNS check routines for the upstream
func (l *lb) initDnsCheck(u *upstream) {
	ln := l.et.String()

	if u.dns.confTtl != 0 {
		LogDVf("LB DNS (%s): using upstream config DNS ttl %ds", u.name, u.dns.confTtl)
		u.dns.ttl = u.dns.confTtl
		u.dns.ticker = time.NewTicker(time.Duration(u.dns.ttl) * time.Second)
	} else {
		if u.dns.ttl == 0 {
			u.dns.ttl = lobbySettings.defaultDnsTtl
		}
		LogDVf("LB DNS (%s): using DNS ttl %ds", u.name, u.dns.ttl)
		u.dns.ticker = time.NewTicker(time.Duration(u.dns.ttl) * time.Second)
	}

	l.state.wg.Add(1)
	go func() {
		defer l.state.wg.Done()

		for {
			select {
			case <-u.dns.chDcStop:
				LogIf("LB DNS (%s): DNS check stop requested", u.name)
				return
			case <-u.dns.ticker.C:

				// This has been included to be able to pause the healthcheck for instance in case of reconfiguration
				l.state.m.Lock()
				LogDVf("LB: '%s' load balancer engine changes are locked", ln)
				// Check if the load balancer is in 'terminate' state
				// if it is skip the healthcheck
				if l.state.t {
					l.state.m.Unlock()
					LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
					continue
				}
				LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
				l.state.m.Unlock()

				ua := u.address
				rua, ttl, err := resolveFqdn(u.host, u.dns.addresses, nil)
				if err != nil {
					LogWf("LB DNS (%s): failed to resolve '%s'", u.name, u.host)
					LogWf(
						"LB DNS (%s): upstream address will be kept on the last known A Record: '%s'",
						u.name,
						ua.String(),
					)
					if u.dns.ttl == 0 {
						u.dns.ttl = lobbySettings.defaultDnsTtl
					}
					LogWf("LB DNS (%s): new DNS query in '%d's", u.name, u.dns.ttl)
					u.dns.ticker.Reset(time.Duration(u.dns.ttl) * time.Second)
					continue
				}
				if !ua.Equal(rua) {
					LogIf(
						"LB DNS (%s): upstream IP address changed based on DNS query from '%s' to '%s'",
						u.name,
						ua,
						rua,
					)

					// In case health check is not active for upstream,
					// set the upstream as available. Upstreams without health checks
					// are always considered to be available unless there were DNS
					// issues during setup or reconfiguration
					if !u.healthCheck.active {
						u.available = true
					}

					// update load balancer upstream
					if err = l.updateUpstream(u, &rua); err != nil {
						LogWf(
							"LB DNS (%s): upstream update request failed. This is not expected and the load balancer might be misconfigured as a result. Manual troubleshooting is likely required. Consider increasing load balancer verbosity to troubleshoot further",
							u.name,
						)
						LogIf("LB DNS (%s): %v", u.name, err)
					}
				}
				if u.dns.confTtl == 0 {
					LogDVf(
						"LB DNS (%s): DNS TTL was not configured to be overriden. Using DNS resolved TTL",
						u.name,
					)
					if ttl != 0 {
						u.dns.ttl = ttl
					} else {
						LogDVf(
							"LB DNS (%s): Resolved DNS TTL was '0'. Using %d this time",
							u.name,
							lobbySettings.defaultDnsTtl,
						)
						u.dns.ttl = lobbySettings.defaultDnsTtl
					}
					u.dns.ticker.Reset(time.Duration(u.dns.ttl) * time.Second)
					LogDf("LB DNS (%s): next DNS check to be performed in %d seconds", u.name, u.dns.ttl)
				}
			}
		}
	}()
}

// reconfig implements the load balancer reconfiguration procedure
// After locking any other load balancer changes,
// it initiates a new load balancer engine and
// requests the previous load balancer engine reconfiguration
// Then it starts the new load balancer DNS and health checks
// Sets the previous load balancer state to 'terminate' and unlocks load balancer changes
// Finally, it requests the previous load balancer DNS and health checks to be stopped
// If the new load balancer engine initialization or
// the reconfiguration returns an error, then the reconfig method doesn't proceeds and
// it unlocks the previous load balancer changes and returns the errLbEngineReconfig error
func (l *lb) reconfig(nl *lb) error {
	ln := l.et.String()
	LogIf("LB: '%s' load balancer engine reconfiguration", ln)

	l.state.m.Lock()
	LogDVf("LB: '%s' load balancer engine changes are locked", ln)

	var err error

	if nl.e, err = newLbEngine(nl.et); err != nil {
		l.state.m.Unlock()
		LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
		return fmt.Errorf("%w: %w", errLbEngineReconfig, err)
	}

	if err = l.e.reconfig(nl); err != nil {
		l.state.m.Unlock()
		LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
		return fmt.Errorf("%w: %w", errLbEngineReconfig, err)
	}

	nl.startChecks()

	l.state.t = true
	l.state.m.Unlock()
	LogDVf("LB: '%s' load balancer engine state set to 'terminate' and changes are unlocked", ln)
	l.stopChecks()

	return nil
}

// updateUpstream performs the necessary tasks to refresh an upstream
// given a new upstream IP address
//   - replaces the upstream address with the new IP address
//   - updates the load balancer list of all upstream IP addresses is updated
//   - calls the LB engine upstream update with a list of unique upstream IP addresses for the load balancer
func (l *lb) updateUpstream(u *upstream, nua *net.IP) error {
	LogIf("LB: update upstream for '%s'", u.name)

	ln := l.et.String()

	// Lock load balancer mutex
	l.state.m.Lock()
	defer l.state.m.Unlock()
	LogDVf("LB: '%s' load balancer engine changes are locked", ln)

	oua := u.address
	LogDf("LB: replacing upstream address from '%s' to '%s'", oua.String(), nua.String())
	u.address = *nua

	LogDf("LB: updating load balancer slice of all upstream IPs")
	if oua != nil {
		LogDVf("LB: replacing upstream IP '%s' with '%s'", oua.String(), nua.String())
		if err := l.replaceUpstreamIps(&oua, nua); err != nil {
			LogWf(
				"Upstream DNS: replace upstream IP failed. This is odd and it could be a bug with impact on the Load Balancer functionality. Please report with full logs of load balancer running with VerboseDebug verbosity. %v",
				err,
			)
			l.addUpstreamIps(*nua)
		}
	} else {
		l.addUpstreamIps(*nua)
	}

	LogDf("LB: refreshing load balancer engine for upstream '%s'", u.name)
	unipl := findUniqueNetIp(l.upstreamIps)
	if err := l.e.updateUpstream(u, &unipl); err != nil {
		LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
		return fmt.Errorf("%w: %w", errLbEngineUpstreamUpdate, err)
	}

	LogDVf("LB: '%s' load balancer engine changes are unlocked", ln)
	return nil
}

// lbInit creates and returns a set of load balancer engines
// It reads the config file, checks for config errors
// and loads the config for each load balancer engine
// It returns a pointer to the new LB instances or an errLbNew error
func lbInit() ([]*lb, error) {
	ls := []*lb{}
	LogIf("LB: config file path: %s", lobbySettings.configFilePath)

	// Initialize Routing Config
	configYaml := ConfigYaml{}

	// Get Routing Config file to byte slice
	configFile, err := os.ReadFile(lobbySettings.configFilePath)
	if err != nil {
		LogIf("LB: Failed to open local config file at '%s'. Will try at system location at '%s' next", lobbySettings.configFilePath, lobbySettings.systemConfigFilePath)
		configFile, err = os.ReadFile(lobbySettings.systemConfigFilePath)
		if err != nil {
			return ls, fmt.Errorf("%w: %w: failed to open config file in '%s' and '%s'", errLbInit, errConfFileOpen, lobbySettings.configFilePath, lobbySettings.systemConfigFilePath)
		}
	}

	// Unmarshal Routing Config YAML file
	if err = yaml.Unmarshal(configFile, &configYaml); err != nil {
		return ls, fmt.Errorf("%w: %w: %w", errLbInit, errConfFileUnmarshal, err)
	}

	// Check configuration
	if err = checkConfig(&configYaml); err != nil {
		return ls, fmt.Errorf("%w: %w", errLbInit, err)
	}

	for _, lbc := range configYaml.LbConfig {
		l := &lb{}
		l.upstreamIps = &[]net.IP{}
		l.state.wg = &sync.WaitGroup{}

		// Parse and load the configuration
		if err := l.getConfig(&lbc); err != nil {
			return ls, fmt.Errorf("%w: %w", errLbInit, err)
		}

		if l.e, err = newLbEngine(l.et); err != nil {
			return ls, fmt.Errorf("%w: %w", errLbInit, err)
		}

		ls = append(ls, l)
	}

	return ls, nil
}

// lbsCompare compares two slices of load balancers based on their lbEngineType and returns:
//   - a map with the load balancers that are on both slices (kept)
//   - a slice with the load balancers that are on nlbs, but not on olbs (added)
//   - a slice with the load balancers that are on olbs, but not on nlbs (removed)
//
// The map has the lbEngineType as key where the value is a slice of two load balancers [0] olbs and [1] nlbs
func lbsCompare(olbs, nlbs []*lb) (*map[lbEngineType][]*lb, []*lb, []*lb) {
	var added []*lb   // holds the lb with the respective lbEngineType found on nlbs, but not on olbs
	var removed []*lb // holds the lb with the respective lbEngineType found on olbs, but not on nlbs

	// map holding the lb with engine type found on nlbs and olbs
	// the key is the lbEngineType and the value is a slice of lb
	// [0] holds the respective olbs lb
	// [1] holds the respective nlbs lb
	kept := make(map[lbEngineType][]*lb)

olbsLoop:
	for _, o := range olbs { // loop through all olbs elements
		for _, n := range nlbs { // for each olbs element, loop through all nlbs elements
			if o.et == n.et { // if olbs element and nlbs element have the same lbEngineType
				var k []*lb         // create a slice of load balancers to hold each lb
				k = append(k, o, n) // add the olbs element and then add the nlbs element
				kept[o.et] = k      // register the new lb slice to the 'kept' map
				LogDVf("LB: load balancer '%s' added to the 'kept' map", o.et.String())
				continue olbsLoop // as a match was found, continue to the next olbs and interrupt nlbs loop
			}
		}
		// as this olbs element lbEngineType wasn't found on any nlbs element:
		removed = append(removed, o)
		LogDVf("LB: load balancer '%s' added to the 'removed' slice", o.et.String())
	}

	// now that we have completed the kept and removed ones, we need to check the ones that are on nlbs and not on olbs (added)
	for _, n := range nlbs { // loop through all nlbs elements
		if _, ok := kept[n.et]; !ok { // if the nlbs element lbEngineType is not present in kept, it means it is not found in olbs
			added = append(added, n)
			LogDVf("LB: load balancer '%s' added to the 'added' slice", n.et.String())
		}
	}

	return &kept, added, removed
}

// lbReconfig is used to manage the reconfiguration of the load balancer engines
// A new slice of load balancers will be initialized, where the config is checked and loaded
// The previous and the new load balancer engine slices are compared to identify which
// load balancer engines have been removed, added or kept
// For the ones that have been kept, it is requested to the load balancer engine to reconfigure them
// The added ones are started and the removed ones are stopped
func lbReconfig(lbs *[]*lb) error {
	LogIf("LB: reconfiguration of all load balancers has been requested")
	LogDf("LB: requesting the initialization of the new load balancers engine set")
	nlbs, err := lbInit()
	if err != nil {
		return fmt.Errorf("%w: %w", errLbReconfig, err)
	}

	// Identifying the load balancers that are on the old and new config,
	// that are on the new config, but are not on the old and
	// that are not on the new config, but are on the old
	toKeep, toAdd, toRem := lbsCompare(*lbs, nlbs)

	for _, l := range *toKeep { // for each lb engine that is on both old and new config
		LogIf("LB: load balancer '%s' configuration will be refreshed", l[0].et.String())
		if err := l[0].reconfig(l[1]); err != nil {
			return fmt.Errorf("%w: %w", errLbReconfig, err)
		}
	}

	if len(toAdd) > 0 {
		for _, l := range toAdd { // for each lb engine that is on the new config, but not on the old
			LogIf("LB: load balancer '%s' has been added to configuration and will be started", l.et.String())
			if err := l.start(); err != nil {
				return fmt.Errorf("%w: %w", errLbReconfig, err)
			}
		}
	}

	if len(toRem) > 0 {
		for _, l := range toRem { // for each lb engine that is on the old config, but not on the new
			LogIf("LB: load balancer '%s' no longer configured will be stopped", l.et.String())
			if err := l.stop(); err != nil {
				return fmt.Errorf("%w: %w", errLbReconfig, err)
			}
		}
	}

	*lbs = nlbs // update var holding load balancers replacing the old with the new one

	return nil
}
