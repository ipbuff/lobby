package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"

	"kernel.org/pub/linux/libs/security/libcap/cap"
)

// settings
var (
	// Generic IPv4 ip_forward setting path
	ipv4FPath = "/proc/sys/net/ipv4/ip_forward"
	// IPv6 forwarding for all interfaces setting path
	ipv6FPath = "/proc/sys/net/ipv6/conf/all/forwarding"
)

// iota constants
type (
	hostType byte // host type
	ipFwd    byte // system IP forwarding state
)

const (
	hostTypeUnknown hostType = iota // undefined
	hostTypeIPv4                    // host in IPv4 format
	hostTypeIPv6                    // host in IPv6 format
	hostTypeFqdn                    // host in FQDN format
)

const (
	ipFwdUnknown ipFwd = iota // undefined
	ipFwdNone                 // no IP forwarding
	ipFwdAll                  // IP forwarding enabled for IPv4 and IPv6
	ipFwdV4Only               // IP forwarding enabled for IPv4
	ipFwdV6Only               // IP forwarding enabled for IPv6
)

// Lookup patterns
const (
	// FQDN regex string according to RFC 1123
	fqdnRegexStringRFC1123 = `^([a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})(\.[a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$` // same as hostnameRegexStringRFC1123 but must contain a non numerical TLD (possibly ending with '.')
)

// Pattern matches
var (
	// parses the fqdnRegexStringRFC1123 regex
	fqdnRegexRFC1123 = regexp.MustCompile(fqdnRegexStringRFC1123)
)

// Common errors
var (
	errCheckIpFwd = errors.New(
		"IP Forwarding check failed",
	)
	errReadIpv4Sett = errors.New(
		"Couldn't read IPv4 forwarding control file",
	)
	errReadIpv6Sett = errors.New(
		"Couldn't read IPv6 forwarding control file",
	)
	errCheckCap = errors.New(
		"capabilities check failed",
	)
	errHostType = errors.New(
		"host type not found",
	)
)

// Linux capabilities are a method to assign specific privileges to a running process
// This function checks if a Linux capability is set for a given process
// An error is returned in case it is not possible to perform the check or in case the capability check fails
func checkCapabilities(cs *cap.Set, vec cap.Flag, val cap.Value) error {
	// check for process cs if val capabilities has the vec flags set
	cf, err := cs.GetFlag(vec, val)
	if err != nil {
		return fmt.Errorf("%w: %w", errCheckCap, err)
	}

	if !cf {
		LogDVf("Check capabilities: failed")
		return fmt.Errorf(
			"%w: '%s' flag not set on '%s' capability",
			errCheckCap,
			vec,
			val,
		)
	}

	LogDVf("Check capabilities: succeeded")
	return nil
}

// checkIpFwd checks if IP forwarding is system enabled
// /proc/sys/net/ipv4/ip_forward is checked for IPv4
// /proc/sys/net/ipv6/conf/all/forwarding for IPv6
// IP forwarding is not checked for individual interfaces
// It is only checked if IP forwarding is generically enabled
// The function returns ipFwdUnkown in case of error
// An error is returned if the config files can't be read
// It returns:
//   - ipFwdNone when IP forwarding is not enabled for IPv4 or IPv6
//   - ipFwdAll when IP forwading is enabled for IPv4 and IPv6
//   - ipFwdV4Only when IP forwarding is enabled for IPv4, but not for IPv6
//   - ipFwdV6Only when IP forwarding is not enabled for IPv4, but is enabled for IPv6
func checkIpFwd() (ipFwd, error) {
	LogDf("IP Fwd check: checking '%s' for IPv4 IP Forwarding", ipv4FPath)
	ipv4f, err := os.ReadFile(ipv4FPath)
	if err != nil {
		return ipFwdUnknown, fmt.Errorf("%w: %w", errCheckIpFwd, errReadIpv4Sett)
	}

	LogDf("IP Fwd check: checking '%s' for IPv6 IP Forwarding", ipv6FPath)
	ipv6f, err := os.ReadFile(ipv6FPath)
	if err != nil {
		return ipFwdUnknown, fmt.Errorf("%w: %w", errCheckIpFwd, errReadIpv6Sett)
	}

	// byte 49 is ASCII '1'
	// file has content 0 when disabled
	// file has content 1 when enabled
	oneAscii := 49
	enabled := byte(oneAscii)

	if ipv4f[0] != enabled {
		LogDf("IP Fwd check: IPv4 Forwarding is not enabled for all interfaces")
		if ipv6f[0] != enabled {
			LogDf("IP Fwd check: IPv6 Forwarding is not enabled for all interfaces")
			return ipFwdNone, nil
		} else {
			LogDf("IP Fwd check: IPv6 Forwarding is enabled for all interfaces")
			return ipFwdV6Only, nil
		}
	}

	LogDf("IP Fwd check: IPv4 Forwarding is enabled for all interfaces")

	if ipv6f[0] != enabled {
		LogDf("IP Fwd check: IPv6 Forwarding is not enabled for all interfaces")
		return ipFwdV4Only, nil
	}

	LogDf("IP Fwd check: IPv6 Forwarding is enabled for all interfaces")

	return ipFwdAll, nil
}

// getHostType returns a hostType from input string
func getHostType(host string) (hostType, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.To4() != nil {
			return hostTypeIPv4, nil
		} else if ip.To16() != nil {
			return hostTypeIPv6, nil
		}
	}

	if isFqdn(host) {
		return hostTypeFqdn, nil
	}

	return hostTypeUnknown, fmt.Errorf("'%s' '%w'", host, errHostType)
}

// isFqdn checks if input string is FQDN and returns boolean result
func isFqdn(s string) bool {
	return fqdnRegexRFC1123.MatchString(s)
}

// findUniqueNetIp returns a list of unique net.IP
func findUniqueNetIp(nipl *[]net.IP) []net.IP {
	unipl := make([]net.IP, 0)

niplLoop:
	for _, nip := range *nipl {
		for _, uip := range unipl {
			if uip.Equal(nip) {
				continue niplLoop
			}
		}
		unipl = append(unipl, nip)
	}

	return unipl
}

// findDuplicateNetIp returns a list of duplicate net.IP
func findDuplicateNetIp(nipl *[]net.IP) []net.IP {
	dnipl := make([]net.IP, 0)
	occurrenceCount := make(map[string]int)

	for _, nip := range *nipl {
		occurrenceCount[nip.String()]++
		if occurrenceCount[nip.String()] == 2 {
			dnipl = append(dnipl, nip)
		}
	}

	return dnipl
}
