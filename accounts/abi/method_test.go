// Copyright 2018 The go-AVNereum Authors
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

package abi

import (
	"strings"
	"testing"
)

const mAVNoddata = `
[
	{"type": "function", "name": "balance", "stateMutability": "view"},
	{"type": "function", "name": "send", "inputs": [{ "name": "amount", "type": "uint256" }]},
	{"type": "function", "name": "transfer", "inputs": [{"name": "from", "type": "address"}, {"name": "to", "type": "address"}, {"name": "value", "type": "uint256"}], "outputs": [{"name": "success", "type": "bool"}]},
	{"constant":false,"inputs":[{"components":[{"name":"x","type":"uint256"},{"name":"y","type":"uint256"}],"name":"a","type":"tuple"}],"name":"tuple","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
	{"constant":false,"inputs":[{"components":[{"name":"x","type":"uint256"},{"name":"y","type":"uint256"}],"name":"a","type":"tuple[]"}],"name":"tupleSlice","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
	{"constant":false,"inputs":[{"components":[{"name":"x","type":"uint256"},{"name":"y","type":"uint256"}],"name":"a","type":"tuple[5]"}],"name":"tupleArray","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
	{"constant":false,"inputs":[{"components":[{"name":"x","type":"uint256"},{"name":"y","type":"uint256"}],"name":"a","type":"tuple[5][]"}],"name":"complexTuple","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},
	{"stateMutability":"nonpayable","type":"fallback"},
	{"stateMutability":"payable","type":"receive"}
]`

func TestMAVNodString(t *testing.T) {
	var table = []struct {
		mAVNod      string
		expectation string
	}{
		{
			mAVNod:      "balance",
			expectation: "function balance() view returns()",
		},
		{
			mAVNod:      "send",
			expectation: "function send(uint256 amount) returns()",
		},
		{
			mAVNod:      "transfer",
			expectation: "function transfer(address from, address to, uint256 value) returns(bool success)",
		},
		{
			mAVNod:      "tuple",
			expectation: "function tuple((uint256,uint256) a) returns()",
		},
		{
			mAVNod:      "tupleArray",
			expectation: "function tupleArray((uint256,uint256)[5] a) returns()",
		},
		{
			mAVNod:      "tupleSlice",
			expectation: "function tupleSlice((uint256,uint256)[] a) returns()",
		},
		{
			mAVNod:      "complexTuple",
			expectation: "function complexTuple((uint256,uint256)[5][] a) returns()",
		},
		{
			mAVNod:      "fallback",
			expectation: "fallback() returns()",
		},
		{
			mAVNod:      "receive",
			expectation: "receive() payable returns()",
		},
	}

	abi, err := JSON(strings.NewReader(mAVNoddata))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range table {
		var got string
		if test.mAVNod == "fallback" {
			got = abi.Fallback.String()
		} else if test.mAVNod == "receive" {
			got = abi.Receive.String()
		} else {
			got = abi.MAVNods[test.mAVNod].String()
		}
		if got != test.expectation {
			t.Errorf("expected string to be %s, got %s", test.expectation, got)
		}
	}
}

func TestMAVNodSig(t *testing.T) {
	var cases = []struct {
		mAVNod string
		expect string
	}{
		{
			mAVNod: "balance",
			expect: "balance()",
		},
		{
			mAVNod: "send",
			expect: "send(uint256)",
		},
		{
			mAVNod: "transfer",
			expect: "transfer(address,address,uint256)",
		},
		{
			mAVNod: "tuple",
			expect: "tuple((uint256,uint256))",
		},
		{
			mAVNod: "tupleArray",
			expect: "tupleArray((uint256,uint256)[5])",
		},
		{
			mAVNod: "tupleSlice",
			expect: "tupleSlice((uint256,uint256)[])",
		},
		{
			mAVNod: "complexTuple",
			expect: "complexTuple((uint256,uint256)[5][])",
		},
	}
	abi, err := JSON(strings.NewReader(mAVNoddata))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range cases {
		got := abi.MAVNods[test.mAVNod].Sig
		if got != test.expect {
			t.Errorf("expected string to be %s, got %s", test.expect, got)
		}
	}
}
