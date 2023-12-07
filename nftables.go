package main

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)

// Some hardcoded settings
const (
	antnSuffixTimeFormat     = "03040502012006"         // time format suffix to be used in the nft table name
	lobbyNftTableNamePattern = `^%s-\d{%d}$`            // nft table name pattern
	nftFamily                = nftables.TableFamilyINet // nft table family. INet means both IPv4 and IPv6
	ugFoModeNftNameSuffix    = "-"                      // Suffix to be used on nftables for upstream groups chain name
)

var (
	// nft table name to be used by nftables for the application
	appNftTableName = lobbySettings.appName
	// default 'postrouting' nftables chain priority
	defaultPostrChainPrio = *nftables.ChainPriorityFilter
	// default 'prerouting' nftables chain priority
	defaultPrerChainPrio = *nftables.ChainPriorityNATDest
	// regex string to match nft table name
	lobbyNftTableNameRegex = fmt.Sprintf(
		lobbyNftTableNamePattern,
		regexp.QuoteMeta(lobbySettings.appName),
		len(antnSuffixTimeFormat),
	)
	// supported lb engine protocols and distribution modes
	nftSuppCapabilities = map[lbProto]map[distMode]bool{
		lbProtoTcp: {
			distModeRR: true,
		},
	}
)

// nftables struct
type nft struct {
	table          *nftables.Table        // nftables table
	postrChain     *nftables.Chain        // nftables 'postrouting' chain
	postrChainPrio nftables.ChainPriority // nftables 'postrouting' chain priority
	prerChain      *nftables.Chain        // nftables 'prerouting' chain
	prerChainPrio  nftables.ChainPriority // nftables 'prerouting' chain priority
	m              sync.Mutex             // nftables changes mutex
}

// NFT errors
var (
	errNftPrep = errors.New(
		"Error while preparing nftables",
	)
	errNftInit = errors.New(
		"Error while initializing nftables",
	)
	errNftReconfig = errors.New(
		"Error while reconfiguring nftables",
	)
	errNftAssert = errors.New(
		"Error when asserting lb engine of type nft",
	)
	errNftNetlinkConn = errors.New(
		"Failed to create a netlink connection. This error is not expected. Check that your system has the nf_tables Linux kernel subsystem available and troubleshoot further",
	)
	errNftFlush = errors.New(
		"Error when requesting a nftables flush",
	)
	errNftListTables = errors.New(
		"Error when listing nft tables",
	)
	errNftAddLbTable = errors.New(
		"Error when creating load balancer nft table",
	)
	errNftAddMasquerade = errors.New(
		"Error when adding masquerade in nftables",
	)
	errNftCleanMasquerade = errors.New(
		"Error when cleaning masquerade rules in nftables",
	)
	errNftUpdateUpstreamChain = errors.New(
		"Error when updating upstream chain",
	)
	errNftUpdateUpstream = errors.New(
		"Error during nftables upstream update",
	)
	errNftStop = errors.New(
		"Error during nftables stop process",
	)
	errNftPerm = errors.New(
		"Error during nft lb engine permissions check",
	)
	errNftPermCap = errors.New(
		"When running as unprivileged user, then the app process capability must have 'e' (Effective) and 'p' (Permitted) flags set for NET_ADMIN and NET_RAW capabilities. On most linux systems this can be set with `setcap 'cap_net_admin,cap_net_raw+ep' /path/to/lobby`.\nRestart the load balancer nft engine after fixing the permissions or re-run the load balancer as a privileged/root user",
	)
	errNftUpdateTarget = errors.New(
		"Error updating target",
	)
)

type nftFunc func(c *nftables.Conn) error // nft management functions declaration used for the pushNft wrapper function

// pushNft is a wrapper function to manage system nftables
// It locks the nft mutex to manage concurrency
// Creates a netlink connection
// Executes the passed nft management functions
// And then nftables.Conn.Flush sends all buffered commands in a single batch
func (n *nft) pushNft(fns ...nftFunc) error {
	// Lock nft mutex
	n.m.Lock()
	defer n.m.Unlock()

	// Netlink connection for querying and modifying nftables
	c, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return fmt.Errorf("%w: %w", errNftNetlinkConn, err)
	}
	defer c.CloseLasting()

	// Call the provided functions with the nftables connection
	for _, fn := range fns {
		fn(c)
	}

	// Push changes to nftables
	if err := c.Flush(); err != nil {
		return fmt.Errorf("%w: %w", errNftFlush, err)
	}

	return nil
}

// prepareNftables prepares the nftables for the load balancer
// It checks if there are nft tables which match the application nft tables name pattern
// If it finds a match it means this could be some leftover from a previous instance
// The leftovers can happen for instance upon some kind of crash or uncontrolled failure
// The function clears any nft tables which match the application nft talbes name pattern
// An errNftPrep error is returned in case of issues when connecting to the netlink,
// listing nft tables or flushing the nft changes
func (n *nft) prepareNftables() error {
	// Get all nftables tables
	prepNftFunc := func(c *nftables.Conn) error {
		tables, err := c.ListTables()
		if err != nil {
			return fmt.Errorf("%w: %w: %w", errNftPrep, errNftListTables, err)
		}

		regex := regexp.MustCompile(lobbyNftTableNameRegex)

		for _, t := range tables {
			if regex.MatchString(t.Name) && t.Family == nftFamily {
				LogDf(
					"NFT: Found nft table '%s' with the table name matching the pattern (%s) lobby uses as nft table name. Deleting the existing table to not interfere",
					t.Name,
					lobbyNftTableNameRegex,
				)
				c.DelTable(t)
			}
		}
		return nil
	}
	n.pushNft(prepNftFunc)

	LogDf("NFT: nftables preparation completed")

	return nil
}

// addLbTable adds a nftables table where the nftables load balancing will be setup
func (n *nft) addLbTable() error {
	// nft table used for load balancing
	addLbTableFunc := func(c *nftables.Conn) error {
		n.table = c.AddTable(&nftables.Table{
			Family: nftFamily,
			Name:   appNftTableName + "-" + time.Now().Format(antnSuffixTimeFormat),
		})

		return nil
	}

	err := n.pushNft(addLbTableFunc)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftAddLbTable, err)
	}

	return nil
}

// start calls startOrReconfig as the same function can be used for either start or reconfig
func (n *nft) start(l *lb) error {
	return n.startOrReconfig(l, false)
}

// startOrReconfig is used to start or reconfig the nftables based on the load balancer current definition
func (n *nft) startOrReconfig(l *lb, refresh bool) error {
	if !refresh {
		LogDf("NFT: nft initialization requested")
		err := n.prepareNftables()
		if err != nil {
			return fmt.Errorf("%w: %w", errNftInit, err)
		}
		n.postrChainPrio = defaultPostrChainPrio
		n.prerChainPrio = defaultPrerChainPrio
	} else {
		LogDf("NFT: nft reconfig requested")
	}

	err := n.addLbTable()
	if err != nil {
		return fmt.Errorf("%w: %w", errNftInit, err)
	}
	LogDf("NFT: added Load Balancer nftable '%s'", n.table.Name)

	// If refresh is true it means we're reconfiguring
	// The priority of the postrouting and prerouting chains will be changed
	// This is done so that during the reconfiguration transition there is no overlap
	// as two tables coexist momentarily
	if refresh {
		// When reconfiguring, we want to insert the new config in parallel with a different priority
		// before clearing the previous config. This is so that at no point in time during the transition
		// the nft is left without either the old or the new config
		if n.postrChainPrio == defaultPostrChainPrio {
			LogDVf(
				"NFT: 'postrouting' chain prio was %d",
				n.postrChainPrio,
			)
			// increment priority by one
			n.postrChainPrio++
			LogDVf(
				"NFT: 'postrouting' chain prio now set to %d",
				n.postrChainPrio,
			)
		} else {
			LogDVf(
				"NFT: 'postrouting'chain prio was %d",
				n.postrChainPrio,
			)
			// reset priority
			n.postrChainPrio = defaultPostrChainPrio
			LogDVf(
				"NFT: 'postrouting' chain prio now set to %d",
				n.postrChainPrio,
			)
		}

		if n.prerChainPrio == defaultPrerChainPrio {
			LogDVf(
				"NFT: 'prerouting' chain prio was %d",
				n.prerChainPrio,
			)
			// increment priority by one
			n.prerChainPrio++
			LogDVf(
				"NFT: 'prerouting' chain prio now set to %d",
				n.prerChainPrio,
			)
		} else {
			LogDVf(
				"NFT: 'prerouting' chain prio was %d",
				n.prerChainPrio,
			)
			n.prerChainPrio = defaultPrerChainPrio
			LogDVf(
				"NFT: 'prerouting' chain prio now set to %d",
				n.prerChainPrio,
			)
		}
	} else {
		LogDf("NFT: nftables startup process")
	}

	setMasqueradeFunc := func(c *nftables.Conn) error {
		// NAT postrouting is required to set masquerade (SNAT) toward targets
		n.postrChain = c.AddChain(&nftables.Chain{
			Name:     "postrouting",
			Table:    n.table,
			Type:     nftables.ChainTypeNAT,
			Hooknum:  nftables.ChainHookPostrouting,
			Priority: &n.postrChainPrio,
		})

		// Create masquerade nft rule for unique upstream IP addresses
		uniqueUpstreamIps := findUniqueNetIp(l.upstreamIps)
		for _, ip := range uniqueUpstreamIps {
			c.AddRule(&nftables.Rule{
				Table: n.table,
				Chain: n.postrChain,
				Exprs: []expr.Any{
					&expr.Meta{
						Key:      expr.MetaKeyNFPROTO,
						Register: 1,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{unix.NFPROTO_IPV4},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       16,
						Len:          4,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     ip.To4(),
					},
					&expr.Masq{},
				},
			})
		}
		return nil
	}

	// NAT prerouting chain is required to set load balancing rules
	addPrerChainFunc := func(c *nftables.Conn) error {
		n.prerChain = c.AddChain(&nftables.Chain{
			Name:     "prerouting",
			Table:    n.table,
			Type:     nftables.ChainTypeNAT,
			Hooknum:  nftables.ChainHookPrerouting,
			Priority: &n.prerChainPrio,
		})
		return nil
	}
	if err = n.pushNft(setMasqueradeFunc, addPrerChainFunc); err != nil {
		if err != nil {
			return fmt.Errorf("%w: %w", errNftInit, err)
		}
	}

	for _, t := range l.targets {
		// Initialize blank nftUgSet, nftUgChain, nftUgChainRule, nftPrerRule
		for i := 0; i < numUgFoModes; i++ {
			t.upstreamGroup.nftUgSet = append(t.upstreamGroup.nftUgSet, &nftables.Set{})
			t.upstreamGroup.nftUgChain = append(t.upstreamGroup.nftUgChain, &nftables.Chain{})
			t.upstreamGroup.nftUgChainRule = append(
				t.upstreamGroup.nftUgChainRule,
				&nftables.Rule{},
			)
			t.nftPrerRule = append(t.nftPrerRule, &nftables.Rule{})
		}

		// Initialize upstreamGroup counter
		t.upstreamGroup.nftCounter = &nftables.CounterObj{
			Table:   n.table,
			Name:    t.name,
			Bytes:   0,
			Packets: 0,
		}

		// Set nftables for target
		if err = n.updateTarget(t); err != nil {
			return fmt.Errorf("%w: %w", errNftInit, err)
		}
	}

	return nil
}

// nftables load balancing is simply stopped by deleting the load balancer nftables table
// where all the load balancing is setup
func (n *nft) stop() error {
	LogIf("NFT: a stop was requested. Initiating nftables cleanup")

	LogDf("NFT: deleting nft table '%s' created for traffic load balancing", n.table.Name)
	err := n.pushNft(func(c *nftables.Conn) error {
		c.DelTable(n.table)
		return nil
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errNftStop, err)
	}

	return nil
}

// getVmapElements returns a list of nftables.SetElement for a given target
// a nftables.SetElement in this context is a nftables veredict to jump to a upstream chain
func getVmapElements(t *target) *[]nftables.SetElement {
	var vmapElements []nftables.SetElement
	activeCount := 0

	for _, u := range t.upstreamGroup.upstreams {
		if u.available {
			vmape := nftables.SetElement{
				Key: binaryutil.NativeEndian.PutUint16(uint16(activeCount)),
				VerdictData: &expr.Verdict{
					Kind:  unix.NFT_JUMP,
					Chain: u.name,
				},
			}

			vmapElements = append(vmapElements, vmape)
			activeCount++
		}
	}

	return &vmapElements
}

// numActiveUpstreams returns the number of active upstreams
func numActiveUpstreams(t *target) uint16 {
	nActiveUpstreams := uint16(0)
	for _, u := range t.upstreamGroup.upstreams {
		if u.available {
			nActiveUpstreams++
		}
	}

	return nActiveUpstreams
}

// updateTarget updates the nftables for a given lb target
func (n *nft) updateTarget(t *target) error {
	LogIf(
		"NFT: Setting nftables for target '%s' (protocol %s on port %d)",
		t.name,
		t.protocol.String(),
		t.port,
	)

	// Lock nft mutex
	n.m.Lock()
	defer n.m.Unlock()

	// Netlink connection for querying and modifying nftables
	c, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return fmt.Errorf("%w: %w", errNftUpdateTarget, errNftNetlinkConn)
	}
	defer c.CloseLasting()

	// Get number of active upstreams
	nActiveUpstreams := numActiveUpstreams(t)
	// Number of configured upstreams
	numUpstreams := uint16(len(t.upstreamGroup.upstreams))
	if nActiveUpstreams == 0 {
		LogIf("NFT: No upstreams available for target '%s'\n", t.name)
		// Given there are no available upstreams, record previous failoverMode and
		// set the failover mode to ugFoModeDown
		t.upstreamGroup.previousFailoverMode = t.upstreamGroup.failoverMode
		t.upstreamGroup.failoverMode = ugFoModeDown
	} else if nActiveUpstreams == numUpstreams {
		LogIf("NFT: All %d upstreams available for target '%s'\n", numUpstreams, t.name)
		// Given all upstreams are up, record the revious failoverMode and
		// set the failover mode to ugFoModeInactive
		t.upstreamGroup.previousFailoverMode = t.upstreamGroup.failoverMode
		t.upstreamGroup.failoverMode = ugFoModeInactive

	} else {
		LogIf("NFT: %d/%d upstreams available for target '%s'\n", nActiveUpstreams, numUpstreams, t.name)
		// Given not all upstreams are available and there are no available upstreams,
		// record previous failoverMode and set the failverMode to the next failoverMode
		t.upstreamGroup.previousFailoverMode = t.upstreamGroup.failoverMode
		t.upstreamGroup.failoverMode, _ = t.upstreamGroup.failoverMode.nextMode()
	}

	ugFM := t.upstreamGroup.failoverMode
	ugName := t.upstreamGroup.name + ugFoModeNftNameSuffix + ugFM.getId()

	// New failover chain
	t.upstreamGroup.nftUgChain[ugFM] = c.AddChain(&nftables.Chain{
		Name:  ugName,
		Table: n.table,
	})

	// Ensure upstream chains exist
	// Needed at nftables initalization
	chains, _ := c.ListChains()
	for _, u := range t.upstreamGroup.upstreams {
		chainFound := false
		for _, chain := range chains {
			if chain.Name == u.name && chain.Table.Name == n.table.Name {
				LogDVf("NFT: Found chain %s in table %s", chain.Name, chain.Table.Name)
				chainFound = true
				break
			} else {
				LogDVf("NFT: no match")
				LogDVf("NFT: nft table name %s chain name %s", chain.Table.Name, chain.Name)
				LogDVf("NFT: config table name %s chain name %s", n.table.Name, u.name)
			}
		}
		if !chainFound {
			LogDf("NFT: Setting up chain for upstream %s in table %s", u.name, n.table.Name)
			LogDf("NFT: Upstream address is %s:%d", u.address.String(), u.port)
			if u.address != nil {
				c.AddRule(&nftables.Rule{
					Table: n.table,
					Chain: c.AddChain(&nftables.Chain{
						Name:  u.name,
						Table: n.table,
					}),
					Exprs: []expr.Any{
						&expr.Immediate{
							Register: 1,
							Data:     u.address.To4(),
						},
						&expr.Immediate{
							Register: 2,
							Data:     binaryutil.BigEndian.PutUint16(u.port),
						},
						&expr.NAT{
							Type:        expr.NATTypeDestNAT,
							Family:      unix.NFPROTO_IPV4,
							RegAddrMin:  1,
							RegAddrMax:  0,
							RegProtoMin: 2,
							RegProtoMax: 0,
						},
					},
				})
			}
		}
	}

	// New set with active upstreams
	t.upstreamGroup.nftUgSet[ugFM] = &nftables.Set{
		Name:     ugName,
		Table:    n.table,
		KeyType:  nftables.TypeInetService,
		DataType: nftables.TypeVerdict,
		IsMap:    true,
	}
	vmapElements := getVmapElements(t)
	c.AddSet(t.upstreamGroup.nftUgSet[ugFM], *vmapElements)

	// New failover chain rule
	if nActiveUpstreams == 0 {
		t.upstreamGroup.nftUgChainRule[ugFM] = c.AddRule(&nftables.Rule{
			Table: n.table,
			Chain: t.upstreamGroup.nftUgChain[ugFM],
			Exprs: []expr.Any{
				&expr.Reject{},
			},
		})
	} else {
		t.upstreamGroup.nftUgChainRule[ugFM] = c.AddRule(&nftables.Rule{
			Table: n.table,
			Chain: t.upstreamGroup.nftUgChain[ugFM],
			Exprs: []expr.Any{
				&expr.Numgen{
					Register: 1,
					Type:     unix.NFT_NG_INCREMENTAL,
					Modulus:  uint32(len(*vmapElements)),
					Offset:   0,
				},
				&expr.Lookup{
					SourceRegister: 1,
					DestRegister:   0,
					SetName:        t.upstreamGroup.nftUgSet[ugFM].Name,
					SetID:          t.upstreamGroup.nftUgSet[ugFM].ID,
					IsDestRegSet:   true,
				},
			},
		})
	}

	// Check if counter objects already exist
	_, err = c.GetObject(t.upstreamGroup.nftCounter)
	if err != nil {
		c.AddObj(t.upstreamGroup.nftCounter)
	}

	// Check if prerouting chain is empty
	// Check is needed at nftables initalization
	if !t.nftRuleInit {
		LogDf(
			"NFT: prerouting chain needs to be initialized for target %s. Redirected to upstream group %s",
			t.name,
			ugName,
		)
		t.nftPrerRule[ugFM] = c.AddRule(&nftables.Rule{
			Table: n.table,
			Chain: n.prerChain,
			Exprs: []expr.Any{
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: 1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte{unix.IPPROTO_TCP},
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     binaryutil.BigEndian.PutUint16(t.port),
				},
				&expr.Objref{
					Type: 1,
					Name: t.upstreamGroup.nftCounter.Name,
				},
				&expr.Verdict{
					Kind:  expr.VerdictKind(unix.NFT_JUMP),
					Chain: ugName,
				},
			},
		})

		t.nftRuleInit = true
	} else {
		// prerouting rule update to new chain
		rules, _ := c.GetRules(n.table, n.prerChain)

		for _, r := range rules {
			if t.nftPrerRule[t.upstreamGroup.previousFailoverMode].Exprs[5].(*expr.Verdict).Chain == r.Exprs[5].(*expr.Verdict).Chain {
				t.nftPrerRule[ugFM] = c.ReplaceRule(&nftables.Rule{
					Table:  n.table,
					Chain:  n.prerChain,
					Handle: r.Handle,
					Exprs: []expr.Any{
						// [ meta load l4proto => reg 1 ]
						&expr.Meta{
							Key:      expr.MetaKeyL4PROTO,
							Register: 1,
						},
						// [ cmp eq reg 1 0x00000006 ]
						&expr.Cmp{
							Op:       expr.CmpOpEq,
							Register: 1,
							Data:     []byte{unix.IPPROTO_TCP},
						},
						// [ payload load 2b @ transport header + 2 => reg 1 ]
						&expr.Payload{
							DestRegister: 1,
							Base:         expr.PayloadBaseTransportHeader,
							Offset:       2,
							Len:          2,
						},
						// [ cmp eq reg 1 0x0000901f ]
						&expr.Cmp{
							Op:       expr.CmpOpEq,
							Register: 1,
							Data:     binaryutil.BigEndian.PutUint16(t.port),
						},
						// [ objref type 1 name counterName ]
						&expr.Objref{
							Type: 1,
							Name: t.upstreamGroup.nftCounter.Name,
						},
						// [ immediate reg 0 jump -> chain ]
						&expr.Verdict{
							Kind:  expr.VerdictKind(unix.NFT_JUMP),
							Chain: t.upstreamGroup.nftUgChain[ugFM].Name,
						},
					},
				})
			}
		}
	}

	// Cleanup previous failover mode chain
	// Loop through all chains is needed to prevent null pointers at nftables initialization
	for _, chain := range chains {
		if chain.Name == t.upstreamGroup.nftUgChain[t.upstreamGroup.previousFailoverMode].Name &&
			chain.Table.Name == n.table.Name {
			c.DelChain(t.upstreamGroup.nftUgChain[t.upstreamGroup.previousFailoverMode])
		}
	}

	// Cleanup previous failover mode set
	// Loop through all sets is needed to prevent null pointers at nftables initialization
	sets, _ := c.GetSets(n.table)
	for _, s := range sets {
		if s.Name == t.upstreamGroup.nftUgSet[t.upstreamGroup.previousFailoverMode].Name &&
			s.Table.Name == n.table.Name {
			c.DelSet(t.upstreamGroup.nftUgSet[t.upstreamGroup.previousFailoverMode])
		}
	}

	if err := c.Flush(); err != nil {
		return fmt.Errorf("%w: %w", errNftUpdateTarget, errNftFlush)
	}

	return nil
}

// addMasquerade adds a masquerade rule on the nftables for a given IP address
// it checks if a masquerade rule already exists for that IP and adds if not
func (n *nft) addMasquerade(ip *net.IP) error {
	LogDf("NFT: Add masquerade for '%s' requested", ip.String())

	addMasqueradeFunc := func(c *nftables.Conn) error {
		pr, err := c.GetRules(
			n.table,
			n.postrChain,
		)
		if err != nil {
			LogWf(
				"NFT: get rules nftables operation failed. This is not expected and the load balancer might be misconfigured as a result. Manual troubleshooting is likely required. Consider increasing load balancer verbosity to troubleshoot further",
			)
		}

		// Check if it is necessary to add masquerade
		needsAdding := true
		for _, rule := range pr {
			rip := net.IP(rule.Exprs[3].(*expr.Cmp).Data)
			if rip.Equal(*ip) {
				needsAdding = false
				break
			}
		}

		if needsAdding {
			LogDVf("NFT: It is necessary to add masquerade")
			// Add masquerade in 'postrouting' chain for upstream IP
			c.AddRule(&nftables.Rule{
				Table: n.table,
				Chain: n.postrChain,
				Exprs: []expr.Any{
					&expr.Meta{
						Key:      expr.MetaKeyNFPROTO,
						Register: 1,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{unix.NFPROTO_IPV4},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       16,
						Len:          4,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     ip.To4(),
					},
					&expr.Masq{},
				},
			})
		} else {
			LogDVf("NFT: It is not necessary to add masquerade as it already exists")
		}
		return nil
	}

	err := n.pushNft(addMasqueradeFunc)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftAddMasquerade, err)
	}

	return nil
}

// cleanMasquerade deletes nftables masquerade rules
// that are not found or that are duplicate for the given slice of net.IP
func (n *nft) cleanMasquerade(lip *[]net.IP) error {
	LogDf("NFT: masquerade rule clean up was requested")

	cleanMasqueradeFunc := func(c *nftables.Conn) error {
		// Get 'postrouting' chain rules
		pr, err := c.GetRules(
			n.table,
			n.postrChain,
		)
		if err != nil {
			LogWf(
				"NFT: get rules nftables operation failed. This is not expected and the load balancer might be misconfigured as a result. Manual troubleshooting is likely required. Consider increasing load balancer verbosity to troubleshoot further",
			)
		}

		// Delete unnecessary masquerade rules
	rulesLoop:
		for _, rule := range pr {
			rip := net.IP(rule.Exprs[3].(*expr.Cmp).Data)
			for _, ip := range *lip {
				if rip.Equal(ip) {
					continue rulesLoop
				}
			}
			LogDVf("NFT: masquerade rule for '%s' is no longer necessary. Deleting", rip.String())
			c.DelRule(rule)
		}

		// Delete duplicate masquerade rules
		dnipl := findDuplicateNetIp(lip)
		for _, dip := range dnipl {
			for _, rule := range pr {
				rip := net.IP(rule.Exprs[3].(*expr.Cmp).Data)
				if dip.Equal(rip) {
					LogDVf("NFT: removing duplicate masquerade entry for '%s'", rip.String())
					c.DelRule(rule)
				}
			}
		}
		return nil
	}

	err := n.pushNft(cleanMasqueradeFunc)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftCleanMasquerade, err)
	}

	return nil
}

// updateUpstreamChain updates the nftables upstream chain
// It checks if the upstream chain rule already exists and adds if not
// Otherwise, it replaces the existing rule
// Upstream chains always have a single rule which is why
// it is ok to [0] the chain rules
func (n *nft) updateUpstreamChain(u *upstream) error {
	LogDf("NFT: update for upstream '%s' chain requested", u.name)

	updateUpstreamChainFunc := func(c *nftables.Conn) error {
		// Get upstream chain rules
		ucr, err := c.GetRules(
			n.table,
			&nftables.Chain{
				Name: u.name,
			},
		)
		if err != nil {
			LogWf(
				"NFT: get rules nftables operation failed. This is not expected and the load balancer might be misconfigured as a result. Manual troubleshooting is likely required. Consider increasing load balancer verbosity to troubleshoot further",
			)
		}

		if len(ucr) != 0 {
			LogDVf("NFT: upstream chain rule already exists. Replacing existing chain")
			c.ReplaceRule(&nftables.Rule{
				Table:  n.table,
				Chain:  ucr[0].Chain,
				Handle: ucr[0].Handle,
				Exprs: []expr.Any{
					&expr.Immediate{
						Register: 1,
						Data:     u.address.To4(),
					},
					&expr.Immediate{
						Register: 2,
						Data:     binaryutil.BigEndian.PutUint16(u.port),
					},
					&expr.NAT{
						Type:        expr.NATTypeDestNAT,
						Family:      unix.NFPROTO_IPV4,
						RegAddrMin:  1,
						RegAddrMax:  0,
						RegProtoMin: 2,
						RegProtoMax: 0,
					},
				},
			})
		} else {
			LogDVf("NFT: upstream chain rule does not exists yet. Adding chain")
			c.AddRule(&nftables.Rule{
				Table: n.table,
				Chain: c.AddChain(&nftables.Chain{
					Name:  u.name,
					Table: n.table,
				}),
				Exprs: []expr.Any{
					&expr.Immediate{
						Register: 1,
						Data:     u.address.To4(),
					},
					&expr.Immediate{
						Register: 2,
						Data:     binaryutil.BigEndian.PutUint16(u.port),
					},
					&expr.NAT{
						Type:        expr.NATTypeDestNAT,
						Family:      unix.NFPROTO_IPV4,
						RegAddrMin:  1,
						RegAddrMax:  0,
						RegProtoMin: 2,
						RegProtoMax: 0,
					},
				},
			})
		}
		return nil
	}

	err := n.pushNft(updateUpstreamChainFunc)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftUpdateUpstreamChain, err)
	}

	return nil
}

// updateUpstream refreshes the upstream nftables rules
// It first adds the masquerade rule for the upstream IP address
// Then it updates the upstream nftables chain
// Lastly, it performs a masquerade rules cleanup
// The cleanup is required to be done only after the upstream chain update
// to ensure that traffic is not disrupted by removing the masquerade before the new
// upstream rules are configured
func (n *nft) updateUpstream(u *upstream, auip *[]net.IP) error {
	LogDf("NFT: update for upstream '%s' requested", u.name)

	// All upstream IPs must have a masquerade rule in 'postrouting' chain
	err := n.addMasquerade(&u.address)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftUpdateUpstream, err)
	}

	// Update upstream chain rules
	err = n.updateUpstreamChain(u)

	// Clean up the masquerade rules
	err = n.cleanMasquerade(auip)
	if err != nil {
		return fmt.Errorf("%w: %w", errNftUpdateUpstream, err)
	}

	return nil
}

// reconfig receives the new load balancer and deals with the nftables transition
// from the previous configuration to the new one
func (n *nft) reconfig(nl *lb) error {
	LogDVf("NFT: nft reconfig was requested")
	nn, ok := nl.e.(*nft) // assert if it is a nftables lb engine
	if !ok {
		return fmt.Errorf("%w: %w", errNftReconfig, errNftAssert)
	}

	// the new postrouting and prerouting nftables priorities are set to the value
	// of the previous load balancer nftables priorities so these can be assessed
	// as part of the reconfig method. This causes the previous config and the
	// new config to run simultaneously, but on different priorities to ensure that there
	// is no interruption to the traffic during the transition between the old and new config
	nn.postrChainPrio = n.postrChainPrio
	nn.prerChainPrio = n.prerChainPrio

	// request a reconfig for the new lb
	if err := nn.startOrReconfig(nl, true); err != nil {
		return fmt.Errorf("%w: %w", errNftReconfig, err)
	}

	// stop the old lb now that the new has been successfully configured
	if err := n.stop(); err != nil {
		return fmt.Errorf("%w: %w", errNftReconfig, err)
	}

	LogDVf("NFT: nft reconfig was successfully completed")
	return nil
}

// getCapabilities provides the nftables supported lb capabilities
func (n *nft) getCapabilities() map[lbProto]map[distMode]bool {
	return nftSuppCapabilities
}

// checkPermissions checks if the minimum required permissions have been granted
// so the load balancer can run successfully. Otherwise, it returns a errCheckPerm error
// CAP_NET_ADMIN and CAP_NET_RAW capabilities are required
// To run as unprivileged user give the required capabilities by
// running the following command as privileged user:
// `setcap 'cap_net_admin,cap_net_raw+ep' <binary_path>`
// Where <binary_path> is this application binary path
// It is possible to check the capabilities of the binary with:
// `getcap <binary_path>`
func (n *nft) checkPermissions() error {
	// Get running process
	cs := cap.GetProc()

	// Capabilities to be checked
	caps := []cap.Value{cap.NET_ADMIN, cap.NET_RAW}
	// Capability set to be checked
	flags := []cap.Flag{cap.Effective, cap.Permitted}

	// For each capability to be checked, check if the capability set is set
	for _, c := range caps {
		LogDVf("NFT: permission check checking '%s' capability", c)
		for _, f := range flags {
			LogDVf("NFT: permission check: checking capability '%s' capability set '%s' ", c, f)
			err := checkCapabilities(cs, f, c)
			if err != nil {
				return fmt.Errorf("%w: %w: \n\n%w\n\n", errNftPerm, err, errNftPermCap)
			}
		}
	}

	LogDf("NFT: permissions check succeeded")
	return nil
}

// This function checks if all the dependencies are satisfied for the nft load balancer to be able to operate successfully
// A check is performed on the IPv4 and IPv6 IP forwarding as this is required for the nftables to be able to process
// traffic to other IP addresses that do not belong to the server which is hosting the Load Balancer
// However, in case it is not confirmed that IPv4 and IPv6 are enabled the Load Balancer an error will not be returned
// It will just log a warning message, because there are scenarios in which the Load Balancer can be used without
// requiring IP packets to be forward to another server
func (n *nft) checkDependencies() error {
	LogDf("Dependencies check: checking if IP forwarding is system enabled")
	ipF, err := checkIpFwd()
	if err != nil {
		return err
	}

	switch ipF {
	case ipFwdUnknown:
		LogWf("Dependencies check: skipped IP forwarding settings check")
	case ipFwdNone:
		LogWf(
			"Dependencies check: IPv4 Forwarding Check Result: FAILED. IPv4 forwarding seems to be generally disabled at system level. This is not a critical error, but the Load Balancer may not function as expected for IPv4 traffic use cases. Make sure the IPv4 forwarding is correctly configured to prevent any issues",
		)
		LogWf(
			"Dependencies check: IPv6 Forwarding Check Result: FAILED. IPv6 forwarding seems to be generally disabled at system level. This is not a critical error, but the Load Balancer may not function as expected for IPv6 traffic use cases. Make sure the IPv6 forwarding is correctly configured to prevent any issues",
		)
	case ipFwdAll:
		LogDf(
			"Dependencies check: IPv4 Forwarding Check Result: SUCCEEDED. IPv4 forwarding seems to be generally enabled",
		)
		LogDf(
			"Dependencies check: IPv6 Forwarding Check Result: SUCCEEDED. IPv6 forwarding seems to be generally enabled",
		)
	case ipFwdV4Only:
		LogDf(
			"Dependencies check: IPv4 Forwarding Check Result: SUCCEEDED. IPv4 forwarding seems to be generally enabled",
		)
		LogWf(
			"Dependencies check: IPv6 Forwarding Check Result: FAILED. IPv6 forwarding seems to be generally disabled at system level. This is not a critical error, but the Load Balancer may not function as expected for IPv6 traffic use cases. Make sure the IPv6 forwarding is correctly configured to prevent any issues",
		)
	case ipFwdV6Only:
		LogWf(
			"Dependencies check: IPv4 Forwarding Check Result: FAILED. IPv4 forwarding seems to be generally disabled at system level. This is not a critical error, but the Load Balancer may not function as expected for IPv4 traffic use cases. Make sure the IPv4 forwarding is correctly configured to prevent any issues",
		)
		LogDf(
			"Dependencies check: IPv6 Forwarding Check Result: SUCCEEDED. IPv6 forwarding seems to be generally enabled",
		)
	}

	LogDf("Dependencies check completed")
	return nil
}
