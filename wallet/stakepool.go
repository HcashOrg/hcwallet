// Copyright (c) 2016 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wallet

import (
	"errors"

	"github.com/HcashOrg/hcd/hcutil"
	"github.com/HcashOrg/hcwallet/wallet/udb"
	"github.com/HcashOrg/hcwallet/walletdb"
)

// StakePoolUserInfo returns the stake pool user information for a user
// identified by their P2SH voting address.
func (w *Wallet) StakePoolUserInfo(userAddress hcutil.Address) (*udb.StakePoolUser, error) {
	switch userAddress.(type) {
	case *hcutil.AddressPubKeyHash: // ok
	case *hcutil.AddressScriptHash: // ok
	default:
		return nil, errors.New("stake pool user address must be P2PKH or P2SH")
	}

	var user *udb.StakePoolUser
	err := walletdb.View(w.db, func(tx walletdb.ReadTx) error {
		stakemgrNs := tx.ReadBucket(wstakemgrNamespaceKey)
		var err error
		user, err = w.StakeMgr.StakePoolUserInfo(stakemgrNs, userAddress)
		return err
	})
	return user, err
}
