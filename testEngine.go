package main

import (
	"net"
)

// supported lb engine protocols and distribution modes
var tlbSuppCapabilities = map[lbProto]map[distMode]bool{
	lbProtoTcp: {
		distModeRR: true,
	},
}

type testLb struct {
	failDependenciesCheck bool
	failPermissionsCheck  bool
	failStart             bool
}

func (tlb *testLb) setResults(failDependenciesCheck, failPermissionsCheck, failStart bool) {
	tlb.failDependenciesCheck = failDependenciesCheck
	tlb.failPermissionsCheck = failPermissionsCheck
	tlb.failStart = failStart
}

func (tlb *testLb) checkDependencies() error {
	if tlb.failDependenciesCheck {
		return errCheckDep
	}
	return nil
}

func (tlb *testLb) checkPermissions() error {
	if tlb.failPermissionsCheck {
		return errCheckPerm
	}
	return nil
}

func (tlb *testLb) getCapabilities() map[lbProto]map[distMode]bool {
	return tlbSuppCapabilities
}

func (tlb *testLb) start(l *lb) error {
	if tlb.failStart {
		return errLbEngineStart
	}
	return nil
}

func (tlb *testLb) stop() error {
	return nil
}

func (tlb *testLb) reconfig(l *lb) error {
	return nil
}

func (tlb *testLb) updateTarget(t *target) error {
	return nil
}

func (tlb *testLb) updateUpstream(u *upstream, auip *[]net.IP) error {
	return nil
}
