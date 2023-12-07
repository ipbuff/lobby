package main

import (
	"errors"
	"net"
	"os"
	"reflect"
	"testing"

	"kernel.org/pub/linux/libs/security/libcap/cap"
)

func TestCheckCapabilities_Failure(t *testing.T) {
	cs := cap.NewSet()

	vna := cap.NET_ADMIN
	fe := cap.Effective

	cs.SetFlag(fe, true, vna)

	err := checkCapabilities(cs, fe, vna)
	if err != nil {
		t.Errorf("expected succeeded: %v", err)
	}
}

func TestCheckCapabilities_Success(t *testing.T) {
	cs := cap.NewSet()

	vna := cap.NET_ADMIN
	fe := cap.Effective
	fp := cap.Permitted

	cs.SetFlag(fe, true, vna)

	err := checkCapabilities(cs, fp, vna)
	if err == nil {
		t.Errorf("expected failed: %v", err)
	}
}

func TestCheckIpFwd(t *testing.T) {
	testZeroPath := "/tmp/lobby_test_zero"
	testOnePath := "/tmp/lobby_test_one"
	testNilPath := "/tmp/lobby_test_nil"
	zero := []byte("0")
	one := []byte("1")
	os.WriteFile(testZeroPath, zero, 0440)
	os.WriteFile(testOnePath, one, 0440)

	exp := map[ipFwd]bool{
		ipFwdUnknown: true,
		ipFwdNone:    false,
		ipFwdAll:     false,
		ipFwdV4Only:  false,
		ipFwdV6Only:  false,
	}

	ipv4FPath = testNilPath
	res, err := checkIpFwd()
	if err == nil {
		t.Errorf(
			"checkIpFwd expected to error due to failure to read ipv4 ip forwarding settings path, but it didn't error",
		)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}

	ipv4FPath = testZeroPath
	ipv6FPath = testNilPath
	res, err = checkIpFwd()
	if err == nil {
		t.Errorf(
			"checkIpFwd expected to error due to failure to read ipv4 ip forwarding settings path, but it didn't error",
		)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}
	exp[ipFwdUnknown] = false

	ipv4FPath = testZeroPath
	ipv6FPath = testZeroPath
	exp[ipFwdNone] = true
	res, err = checkIpFwd()
	if err != nil {
		t.Errorf("checkIpFwd errored unexpectedly: '%v'", err)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}
	exp[ipFwdNone] = false

	ipv4FPath = testOnePath
	ipv6FPath = testZeroPath
	exp[ipFwdV4Only] = true
	res, err = checkIpFwd()
	if err != nil {
		t.Errorf("checkIpFwd errored unexpectedly: '%v'", err)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}
	exp[ipFwdV4Only] = false

	ipv4FPath = testZeroPath
	ipv6FPath = testOnePath
	exp[ipFwdV6Only] = true
	res, err = checkIpFwd()
	if err != nil {
		t.Errorf("checkIpFwd errored unexpectedly: '%v'", err)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}
	exp[ipFwdV6Only] = false

	ipv4FPath = testOnePath
	ipv6FPath = testOnePath
	exp[ipFwdAll] = true
	res, err = checkIpFwd()
	if err != nil {
		t.Errorf("checkIpFwd errored unexpectedly: '%v'", err)
	}

	if check, ok := exp[res]; !ok {
		t.Errorf(
			"checkIpFwd result not found in expected IP Forwarding system settings. Res: '%v'",
			res,
		)
	} else {
		if !check {
			t.Errorf("unexpected ipFwd iota const returned")
		}
	}
	exp[ipFwdAll] = false

	os.Remove(testZeroPath)
	os.Remove(testOnePath)
}

func TestGetHostType(t *testing.T) {
	testCases := []struct {
		input  string
		err    error
		result hostType
	}{
		{input: "1.1.1.1", err: nil, result: hostTypeIPv4},
		{input: "dead:beef::1", err: nil, result: hostTypeIPv6},
		{input: "example.com.", err: nil, result: hostTypeFqdn},
		{input: "example.com", err: nil, result: hostTypeFqdn},
		{input: "11..12.1.", err: errHostType, result: hostTypeUnknown},
		{input: "1111.1.1.1", err: errHostType, result: hostTypeUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			ht, err := getHostType(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected %v, but got %v", tc.input, tc.err, err)
			}
			if ht != tc.result {
				t.Errorf("%s: expected %v, but got %v", tc.input, tc.result, ht)
			}
		})
	}
}

func TestIsFqdn(t *testing.T) {
	testCases := []struct {
		input  string
		result bool
	}{
		{input: "example.com", result: true},
		{input: "example.com.", result: true},
		{input: "testing.example.com", result: true},
		{input: "1.1.1.1", result: false},
		{input: "testing..example.com", result: false},
		{input: "testing@example.com", result: false},
		{input: "testing,example.com", result: false},
		{input: ".com", result: false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			r := isFqdn(tc.input)
			if r != tc.result {
				t.Errorf("%s: expected %v, but got %v", tc.input, tc.result, r)
			}
		})
	}
}

func TestFindUniqueNetIp(t *testing.T) {
	testCases := []struct {
		input  []net.IP
		result []net.IP
	}{
		{
			input: []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.1"),
			},
			result: []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
			},
		},
		{
			input: []net.IP{
				net.ParseIP("2001:db8::1"),
				net.ParseIP("2001:db8::2"),
				net.ParseIP("2001:db8::1"),
			},
			result: []net.IP{
				net.ParseIP("2001:db8::1"),
				net.ParseIP("2001:db8::2"),
			},
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := findUniqueNetIp(&tc.input)
			equal := reflect.DeepEqual(result, tc.result)
			if !equal {
				t.Errorf("For input %v, expected %v, but got %v", tc.input, tc.result, result)
			}
		})
	}
}

func TestFindDuplicateNetIp(t *testing.T) {
	testCases := []struct {
		input  []net.IP
		result []net.IP
	}{
		{
			input: []net.IP{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.1"),
			},
			result: []net.IP{
				net.ParseIP("192.168.0.1"),
			},
		},
		{
			input: []net.IP{
				net.ParseIP("2001:db8::1"),
				net.ParseIP("2001:db8::2"),
				net.ParseIP("2001:db8::1"),
			},
			result: []net.IP{
				net.ParseIP("2001:db8::1"),
			},
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := findDuplicateNetIp(&tc.input)
			equal := reflect.DeepEqual(result, tc.result)
			if !equal {
				t.Errorf("For input %v, expected %v, but got %v", tc.input, tc.result, result)
			}
		})
	}
}
