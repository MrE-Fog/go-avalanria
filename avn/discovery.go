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

package avn

import (
	"github.com/avalanria/go-avalanria/core"
	"github.com/avalanria/go-avalanria/core/forkid"
	"github.com/avalanria/go-avalanria/p2p/enode"
	"github.com/avalanria/go-avalanria/rlp"
)

// avnEntry is the "avn" ENR entry which advertises avn protocol
// on the discovery network.
type avnEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e avnEntry) ENRKey() string {
	return "avn"
}

// startEthEntryUpdate starts the ENR updater loop.
func (avn *Avalanria) startEthEntryUpdate(ln *enode.LocalNode) {
	var newHead = make(chan core.ChainHeadEvent, 10)
	sub := avn.blockchain.SubscribeChainHeadEvent(newHead)

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-newHead:
				ln.Set(avn.currentEthEntry())
			case <-sub.Err():
				// Would be nice to sync with avn.Stop, but there is no
				// good way to do that.
				return
			}
		}
	}()
}

func (avn *Avalanria) currentEthEntry() *avnEntry {
	return &avnEntry{ForkID: forkid.NewID(avn.blockchain.Config(), avn.blockchain.Genesis().Hash(),
		avn.blockchain.CurrentHeader().Number.Uint64())}
}
