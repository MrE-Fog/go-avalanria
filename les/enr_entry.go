// Copyright 2019 The go-AVNereum Authors
// This file is part of the go-AVNereum library.
//
// The go-AVNereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-AVNereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-AVNereum library. If not, see <http://www.gnu.org/licenses/>.

package les

import (
	"github.com/AVNereum/go-AVNereum/core/forkid"
	"github.com/AVNereum/go-AVNereum/p2p/dnsdisc"
	"github.com/AVNereum/go-AVNereum/p2p/enode"
	"github.com/AVNereum/go-AVNereum/rlp"
)

// lesEntry is the "les" ENR entry. This is set for LES servers only.
type lesEntry struct {
	// Ignore additional fields (for forward compatibility).
	VfxVersion uint
	Rest       []rlp.RawValue `rlp:"tail"`
}

func (lesEntry) ENRKey() string { return "les" }

// AVNEntry is the "AVN" ENR entry. This is redeclared here to avoid depending on package AVN.
type AVNEntry struct {
	ForkID forkid.ID
	Tail   []rlp.RawValue `rlp:"tail"`
}

func (AVNEntry) ENRKey() string { return "AVN" }

// setupDiscovery creates the node discovery source for the AVN protocol.
func (AVN *LightAvalanria) setupDiscovery() (enode.Iterator, error) {
	it := enode.NewFairMix(0)

	// Enable DNS discovery.
	if len(AVN.config.EthDiscoveryURLs) != 0 {
		client := dnsdisc.NewClient(dnsdisc.Config{})
		dns, err := client.NewIterator(AVN.config.EthDiscoveryURLs...)
		if err != nil {
			return nil, err
		}
		it.AddSource(dns)
	}

	// Enable DHT.
	if AVN.udpEnabled {
		it.AddSource(AVN.p2pServer.DiscV5.RandomNodes())
	}

	forkFilter := forkid.NewFilter(AVN.blockchain)
	iterator := enode.Filter(it, func(n *enode.Node) bool { return nodeIsServer(forkFilter, n) })
	return iterator, nil
}

// nodeIsServer checks whAVNer n is an LES server node.
func nodeIsServer(forkFilter forkid.Filter, n *enode.Node) bool {
	var les lesEntry
	var AVN AVNEntry
	return n.Load(&les) == nil && n.Load(&AVN) == nil && forkFilter(AVN.ForkID) == nil
}
