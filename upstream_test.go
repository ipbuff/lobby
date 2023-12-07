package main

import (
	"errors"
	"testing"
)

func TestGetHcProto(t *testing.T) {
	testCases := []struct {
		input  string
		err    error
		result hcProto
	}{
		{input: "tcp", err: nil, result: hcProtoTcp},
		{input: "udp", err: nil, result: hcProtoUdp},
		{input: "sctp", err: nil, result: hcProtoSctp},
		{input: "http", err: nil, result: hcProtoHttp},
		{input: "grpc", err: nil, result: hcProtoGrpc},
		{input: "misteak", err: errHcp, result: hcProtoUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			hcp, err := getHcProto(tc.input)
			if !errors.Is(err, tc.err) {
				t.Errorf("%s: expected %v, but got %v", tc.input, tc.err, err)
			}
			if hcp != tc.result {
				t.Errorf("%s: expected %v, but got %v", tc.input, tc.result, hcp)
			}
		})
	}
}

func TestHcPString(t *testing.T) {
	testCases := []struct {
		input  hcProto
		result string
	}{
		{input: hcProtoTcp, result: "tcp"},
		{input: hcProtoUdp, result: "udp"},
		{input: hcProtoSctp, result: "sctp"},
		{input: hcProtoHttp, result: "http"},
		{input: hcProtoGrpc, result: "grpc"},
		{input: hcProtoUnknown, result: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.result, func(t *testing.T) {
			r := tc.input.String()
			if r != tc.result {
				t.Errorf("%s: expected %v, but got %v", tc.result, tc.result, r)
			}
		})
	}
}
func TestGetId(t *testing.T) {
	testCases := []struct {
		input  ugFoMode
		result string
	}{
		{input: ugFoModeUnknown, result: "0"},
		{input: ugFoModeInactive, result: "1"},
		{input: ugFoModeActive1, result: "2"},
		{input: ugFoModeActive2, result: "3"},
		{input: ugFoModeDown, result: "4"},
		{input: 9, result: "0"},
	}

	for _, tc := range testCases {
		t.Run(tc.result, func(t *testing.T) {
			ugFM := tc.input.getId()
			if ugFM != tc.result {
				t.Errorf("%v: expected %v, but got %v", tc.result, tc.input, ugFM)
			}
		})
	}
}

func TestNextMode(t *testing.T) {
	testCases := []struct {
		input  ugFoMode
		err    error
		result ugFoMode
	}{
		{input: ugFoModeUnknown, err: errUgFM, result: ugFoModeUnknown},
		{input: ugFoModeInactive, err: nil, result: ugFoModeActive1},
		{input: ugFoModeActive1, err: nil, result: ugFoModeActive2},
		{input: ugFoModeActive2, err: nil, result: ugFoModeActive1},
		{input: ugFoModeDown, err: nil, result: ugFoModeActive1},
		{input: 9, err: errUgFM, result: ugFoModeUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input.getId(), func(t *testing.T) {
			ugFM, err := tc.input.nextMode()
			if ugFM != tc.result {
				t.Errorf(
					"%v: expected %v, but got %v",
					tc.input.getId(),
					tc.input.getId(),
					ugFM.getId(),
				)
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("%v: expected %v, but got %v", tc.input.getId(), tc.err, err)
			}
		})
	}
}
