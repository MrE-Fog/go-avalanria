// Copyright 2019 The go-avalanria Authors
// This file is part of the go-avalanria library.
//
// The go-avalanria library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-avalanria library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-avalanria library. If not, see <http://www.gnu.org/licenses/>.

package les

import (
	"github.com/avalanria/go-avalanria/core/forkid"
	"github.com/avalanria/go-avalanria/p2p/dnsdisc"
	"github.com/avalanria/go-avalanria/p2p/enode"
	"github.com/avalanria/go-avalanria/rlp"
)

// lesEntry is the "les" ENR entry. This is set for LES servers only.
type lesEntry struct {
	// Ignore additional fields (for forward compatibility).
	VfxVersion uint
	Rest       []rlp.RawValue `rlp:"tail"`
}

func (lesEntry) ENRKey() string { return "les" }

// avnEntry is the "avn" ENR entry. This is redeclared here to avoid depending on package avn.
type avnEntry struct {
	ForkID forkid.ID
	Tail   []rlp.RawValue `rlp:"tail"`
}

func (avnEntry) ENRKey() string { return "avn" }

// setupDiscovery creates the node discovery source for the avn protocol.
func (avn *LightAvalanria) setupDiscovery() (enode.Iterator, error) {
	it := enode.NewFairMix(0)

	// Enable DNS discovery.
	if len(avn.config.EthDiscoveryURLs) != 0 {
		client := dnsdisc.NewClient(dnsdisc.Config{})
		dns, err := client.NewIterator(avn.config.EthDiscoveryURLs...)
		if err != nil {
			return nil, err
		}
		it.AddSource(dns)
	}

	// Enable DHT.
	if avn.udpEnabled {
		it.AddSource(avn.p2pServer.DiscV5.RandomNodes())
	}

	forkFilter := forkid.NewFilter(avn.blockchain)
	iterator := enode.Filter(it, func(n *enode.Node) bool { return nodeIsServer(forkFilter, n) })
	return iterator, nil
}

// nodeIsServer checks whavner n is an LES server node.
func nodeIsServer(forkFilter forkid.Filter, n *enode.Node) bool {
	var les lesEntry
	var avn avnEntry
	return n.Load(&les) == nil && n.Load(&avn) == nil && forkFilter(avn.ForkID) == nil
}
