package main

import (
	"github.com/google/nftables"
)

// A target declares the packet destination as it arrives to the load balancer
type target struct {
	name          string
	protocol      lbProto
	ip            string
	port          uint16
	upstreamGroup *upstreamGroup
	nftRuleInit   bool
	nftPrerRule   []*nftables.Rule
}
