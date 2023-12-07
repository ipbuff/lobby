package main

import (
	"net"
	"testing"
)

func TestResolveFqdnWithDnsAddresses_Success(t *testing.T) {
	fqdn := "example.com."
	dnsa := []string{"8.8.8.8", "8.8.4.4"}
	ip, ttl, err := resolveFqdn(fqdn, dnsa, nil)
	if err != nil {
		t.Errorf("Error resolving FQDN: %v", err)
	}
	t.Logf("Resolved IP: %s, TTL: %d", ip, ttl)
}

func TestResolveFqdnWithDnsAddresses_Failure(t *testing.T) {
	fqdn := "example.5368235687."
	dnsa := []string{"222.222.222.222", "8.8.8.8", "8.8.4.4"}
	ip, ttl, err := resolveFqdn(fqdn, dnsa, nil)
	if err == nil {
		t.Errorf("Succeded, but should have failed: %v", err)
	}
	t.Logf("Resolved IP: %s, TTL: %d", ip, ttl)
}

func TestResolveFqdnWithoutDnsAddresses(t *testing.T) {
	fqdn := "example.com."
	ip, ttl, err := resolveFqdn(fqdn, nil, nil)
	if err != nil {
		t.Errorf("Error resolving FQDN: %v", err)
	}
	t.Logf("Resolved IP: %s, TTL: %d", ip, ttl)
}

func TestResolveFqdWrongFqdn(t *testing.T) {
	fqdn := ".com"
	_, _, err := resolveFqdn(fqdn, nil, nil)
	if err == nil {
		t.Errorf("Expected error due to erroneous FQDN, but didn't get it")
	}
}

func TestResolveFqdnWithCustomResolver(t *testing.T) {
	fqdn := "example.com."
	dnsa := []string{"8.8.8.8", "8.8.4.4"}
	mockResolver := func(f string, dnsa []string) (net.IP, uint32, error) {
		return net.ParseIP("1.1.1.1"), 600, nil
	}
	ip, ttl, _ := resolveFqdn(fqdn, dnsa, mockResolver)
	t.Logf("Resolved IP: %s, TTL: %d", ip, ttl)
}
