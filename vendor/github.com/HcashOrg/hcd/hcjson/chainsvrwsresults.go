// Copyright (c) 2015 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers 
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package hcjson

// SessionResult models the data from the session command.
type SessionResult struct {
	SessionID uint64 `json:"sessionid"`
}

// RescanResult models the result object returned by the rescan RPC.
type RescanResult struct {
	DiscoveredData []RescannedBlock `json:"discovereddata"`
}

// RescannedBlock contains the hash and all discovered transactions of a single
// rescanned block.
type RescannedBlock struct {
	Hash         string   `json:"hash"`
	Transactions []string `json:"transactions"`
}
