// Copyright (c) 2016 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wallet

import (
	"errors"

	"github.com/HcashOrg/hcd/hcutil"
	"github.com/HcashOrg/hcd/txscript"
	"github.com/HcashOrg/hcd/wire"
	"github.com/HcashOrg/hcwallet/apperrors"
	"github.com/HcashOrg/hcwallet/wallet/udb"
	"github.com/HcashOrg/hcwallet/walletdb"
)

// MakeMultiSigScript creates a multi-signature script that can be
// redeemed with nRequired signatures of the passed keys and addresses.  If the
// address is a P2PKH address, the associated pubkey is looked up by the wallet
// if possible, otherwise an error is returned for a missing pubkey.
//
// This function only works with secp256k1 pubkeys and P2PKH addresses derived
// from them.
func (w *Wallet) MakeMultiSigScript(addrs []hcutil.Address, nRequired int) ([]byte, error) {
	return txscript.MultiSigScript(addrs, nRequired)
}

// ImportP2SHRedeemScript adds a P2SH redeem script to the wallet.
func (w *Wallet) ImportP2SHRedeemScript(script []byte) (*hcutil.AddressScriptHash, error) {
	var p2shAddr *hcutil.AddressScriptHash
	err := walletdb.Update(w.db, func(tx walletdb.ReadWriteTx) error {
		addrmgrNs := tx.ReadWriteBucket(waddrmgrNamespaceKey)
		txmgrNs := tx.ReadWriteBucket(wtxmgrNamespaceKey)

		err := w.TxStore.InsertTxScript(txmgrNs, script)
		if err != nil {
			return err
		}

		addrInfo, err := w.Manager.ImportScript(addrmgrNs, script)
		if err != nil {
			// Don't care if it's already there, but still have to
			// set the p2shAddr since the address manager didn't
			// return anything useful.
			if apperrors.IsError(err, apperrors.ErrDuplicateAddress) {
				// This function will never error as it always
				// hashes the script to the correct length.
				p2shAddr, _ = hcutil.NewAddressScriptHash(script,
					w.chainParams)
				return nil
			}
			return err
		}

		p2shAddr = addrInfo.Address().(*hcutil.AddressScriptHash)
		return nil
	})
	return p2shAddr, err
}

// FetchP2SHMultiSigOutput fetches information regarding a wallet's P2SH
// multi-signature output.
func (w *Wallet) FetchP2SHMultiSigOutput(outPoint *wire.OutPoint) (*P2SHMultiSigOutput, error) {
	var (
		mso          *udb.MultisigOut
		redeemScript []byte
	)
	err := walletdb.View(w.db, func(tx walletdb.ReadTx) error {
		txmgrNs := tx.ReadBucket(wtxmgrNamespaceKey)
		var err error

		mso, err = w.TxStore.GetMultisigOutput(txmgrNs, outPoint)
		if err != nil {
			return err
		}

		redeemScript, err = w.TxStore.GetTxScript(txmgrNs, mso.ScriptHash[:])
		if err != nil {
			return err
		}
		// returns nil, nil when it successfully found no script.  That error is
		// only used to return early when the database is closed.
		if redeemScript == nil {
			return errors.New("script not found")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	p2shAddr, err := hcutil.NewAddressScriptHashFromHash(
		mso.ScriptHash[:], w.chainParams)
	if err != nil {
		return nil, err
	}

	multiSigOutput := P2SHMultiSigOutput{
		OutPoint:     *mso.OutPoint,
		OutputAmount: mso.Amount,
		ContainingBlock: BlockIdentity{
			Hash:   mso.BlockHash,
			Height: int32(mso.BlockHeight),
		},
		P2SHAddress:  p2shAddr,
		RedeemScript: redeemScript,
		M:            mso.M,
		N:            mso.N,
		Redeemer:     nil,
	}

	if mso.Spent {
		multiSigOutput.Redeemer = &OutputRedeemer{
			TxHash:     mso.SpentBy,
			InputIndex: mso.SpentByIndex,
		}
	}

	return &multiSigOutput, nil
}

// FetchAllRedeemScripts returns all P2SH redeem scripts saved by the wallet.
func (w *Wallet) FetchAllRedeemScripts() ([][]byte, error) {
	var redeemScripts [][]byte
	err := walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
		txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)
		var err error
		redeemScripts, err = w.TxStore.StoredTxScripts(txmgrNs)
		return err
	})
	return redeemScripts, err
}
