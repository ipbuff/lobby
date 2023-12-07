package main

import (
	"errors"
	"net"

	"github.com/miekg/dns"
)

// hardcoded settings
const (
	// default DNS client config file
	defaultDnsClientConfigFile = "/etc/resolv.conf"
)

var (
	defaultDnsResolver = miekgResolver
)

// DNS errors
var (
	errDnsHostRecordFail = errors.New(
		"Couldn't resolve host record. Check your DNS",
	)
	errDnsHostFqdnCheckFail = errors.New(
		"Host is not valid fqdn",
	)
)

type resolver func(string, []string) (net.IP, uint32, error)

// A wrapper method to a resolver func
// In case a resolver func is not provided, it will call the defaultDnsResolver
func resolveFqdn(f string, dnsa []string, r resolver) (net.IP, uint32, error) {
	if r == nil {
		return defaultDnsResolver(f, dnsa)
	} else {
		return r(f, dnsa)
	}
}

// This func receives a FQDN (canonical names with a trailing dot) and a slice of DNS addresses
// It performs a A record DNS query of the provided FQDN on the supplied DNS addresses
// It will iterate through the DNS Addresses in sequential order until it gets a response or the list ends
// The function returns the first IP address from the response, the DNS TTL and an error
// It returns a non-null error in case a FQDN hasn't been provided or if the query fails on all provided DNS
func miekgResolver(f string, dnsa []string) (net.IP, uint32, error) {
	if !dns.IsFqdn(f) {
		LogDVf("DNS: '%s' is not a valid FQDN", f)
		return nil, 0, errDnsHostFqdnCheckFail
	}
	LogDVf("DNS: '%s' is a valid FQDN", f)

	if len(dnsa) == 0 {
		LogDf("DNS: no DNS addresses have been provided. Reading '%s'", defaultDnsClientConfigFile)

		dnsCfg, _ := dns.ClientConfigFromFile(defaultDnsClientConfigFile)
		dnsa = dnsCfg.Servers

	}

	// Create DNS Client
	c := new(dns.Client)
	// Create DNS Query message
	m := new(dns.Msg)
	m.SetQuestion(f, dns.TypeA)

	// Iterate through each provided DNS address.
	// Until the end of the list,
	// unless a successful response is found.
	for _, ns := range dnsa {
		// Send DNS query
		in, _, err := c.Exchange(m, ns+":"+"53")
		if err != nil {
			LogDVf("DNS: query failed. DNS: '%s' FQDN: '%s'", ns, f)
			continue
		}
		if len(in.Answer) == 0 {
			LogDVf("DNS: couldn't resolve. DNS: '%s' FQDN: '%s'", ns, f)
			continue
		}
		for _, ans := range in.Answer {
			if a, ok := ans.(*dns.A); ok {
				ttl := ans.Header().Ttl
				LogDVf(
					"DNS: resolved successfully. DNS: '%s', FQDN: '%s', A Record: '%s', TTL: '%d'",
					ns,
					f,
					a.A.String(),
					ttl,
				)
				return a.A, ttl, nil
			}
		}
	}

	LogDVf("DNS: none of the provided DNS's was able to resolve '%s'", f)

	return nil, 0, errDnsHostRecordFail
}
