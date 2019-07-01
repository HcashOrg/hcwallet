// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wallet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"encoding/hex"

	"github.com/HcashOrg/hcd/blockchain/stake"
	"github.com/HcashOrg/hcd/chaincfg/chainhash"
	"github.com/HcashOrg/hcd/hcjson"
	"github.com/HcashOrg/hcd/hcutil"
	"github.com/HcashOrg/hcd/txscript"
	"github.com/HcashOrg/hcd/wire"
	"github.com/HcashOrg/hcwallet/apperrors"
	"github.com/HcashOrg/hcwallet/chain"
	"github.com/HcashOrg/hcwallet/omnilib"
	"github.com/HcashOrg/hcwallet/wallet/txrules"
	"github.com/HcashOrg/hcwallet/wallet/udb"
	"github.com/HcashOrg/hcwallet/walletdb"
)

func (w *Wallet) handleConsensusRPCNotifications(chainClient *chain.RPCClient) {
	for n := range chainClient.Notifications() {
		var notificationName string
		var err error
		switch n := n.(type) {
		case chain.ClientConnected:
			log.Infof("The client has successfully connected to hcd and " +
				"is now handling websocket notifications")
		case chain.BlockConnected:
			notificationName = "blockconnected"
			err = w.onBlockConnected(n.BlockHeader, n.Transactions)
			go func(transactions [][]byte) {
				for _, serializedTx := range transactions {
					msgTx:=wire.NewMsgTx()
					err:=msgTx.FromBytes(serializedTx)
					if err != nil {
						str := "failed to deserialize transaction"
						log.Infof(str)
						return
					}

					txHash:=msgTx.TxHash()
					if _,exist:=w.AiTxConfirms[txHash];exist{
						delete(w.AiTxConfirms,txHash)
					}
				}

			}(n.Transactions)
			if err == nil {
				err = walletdb.View(w.db, func(tx walletdb.ReadTx) error {
					return w.watchFutureAddresses(tx)
				})
			}
		case chain.Reorganization:
			notificationName = "reorganizing"
			err = w.handleReorganizing(n.OldHash, n.NewHash, n.OldHeight, n.NewHeight)
		case chain.RelevantTxAccepted:
			notificationName = "relevanttxaccepted"
			var rpt *chainhash.Hash
			rpt, err = w.RescanPoint()
			if err != nil || rpt != nil {
				break
			}

			log.Error("handleConsensusRPCNotifications:", n.Transaction)
			err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
				return w.processSerializedTransaction(dbtx, n.Transaction, nil, nil)
			})
			if err == nil {
				err = walletdb.View(w.db, func(tx walletdb.ReadTx) error {
					return w.watchFutureAddresses(tx)
				})
			}
		case chain.NewInstantTx:
			notificationName = "newinstanttx"
			w.handleNewInstantTx(n.InstantTx, n.Tickets,n.Resend)
		case chain.InstantTxVote:
			notificationName="instanttxvote"
			w.handleInstantTxVote(n.InstantTxVoteHash,n.InstantTxHash,n.TickeHash,n.Vote,n.Sig)
		case chain.MissedTickets:
			notificationName = "spentandmissedtickets"
			err = w.handleMissedTickets(n.BlockHash, int32(n.BlockHeight), n.Tickets)
		}
		if err != nil {
			log.Errorf("Failed to process consensus server notification "+
				"(name: `%s`, detail: `%v`)", notificationName, err)
			//refresh wallet data
			var height int32 = 0
			err = walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
				ns := dbtx.ReadBucket(wtxmgrNamespaceKey)
				_, height = w.TxStore.MainChainTip(ns)
				return nil
			})
			if err == nil && !w.IsScanning()  && w.chainClient != nil {
				w.RescanFromHeight(w.chainClient.Client, height)
			}
		}
	}
}

// AssociateConsensusRPC associates the wallet with the consensus JSON-RPC
// server and begins handling all notifications in a background goroutine.  Any
// previously associated client, if it is a different instance than the passed
// client, is stopped.
func (w *Wallet) AssociateConsensusRPC(chainClient *chain.RPCClient) {
	w.chainClientLock.Lock()
	defer w.chainClientLock.Unlock()
	if w.chainClient != nil {
		if w.chainClient != chainClient {
			w.chainClient.Stop()
		}
	}

	w.chainClient = chainClient

	w.wg.Add(1)
	go func() {
		w.handleConsensusRPCNotifications(chainClient)
		w.wg.Done()
	}()
}

// handleChainNotifications is the major chain notification handler that
// receives websocket notifications about the blockchain.
func (w *Wallet) handleChainNotifications(chainClient *chain.RPCClient) {
	// At the moment there is no recourse if the rescan fails for
	// some reason, however, the wallet will not be marked synced
	// and many methods will error early since the wallet is known
	// to be out of date.
	err := w.syncWithChain(chainClient.Client)
	if err != nil && !w.ShuttingDown() {
		log.Warnf("Unable to synchronize wallet to chain: %v", err)
	}

	w.handleConsensusRPCNotifications(chainClient)
	w.wg.Done()
}

func (w *Wallet) extendMainChain(dbtx walletdb.ReadWriteTx, block *udb.BlockHeaderData, transactions [][]byte) error {
	txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)

	log.Infof("Connecting block %v, height %v", block.BlockHash, block.SerializedHeader.Height())

	err := w.TxStore.ExtendMainChain(txmgrNs, block)
	if err != nil {
		// Propagate the error unless this block is already included in the main
		// chain.
		if !apperrors.IsError(err, apperrors.ErrDuplicate) {
			return err
		}
	}

	// Notify interested clients of the connected block.
	var header wire.BlockHeader
	err = header.Deserialize(bytes.NewReader(block.SerializedHeader[:]))
	if err != nil {
		return err
	}
	w.NtfnServer.notifyAttachedBlock(dbtx, &header, &block.BlockHash)

	blockMeta, err := w.TxStore.GetBlockMetaForHash(txmgrNs, &block.BlockHash)
	if err != nil {
		return err
	}

	for _, serializedTx := range transactions {
		err = w.processSerializedTransaction(dbtx, serializedTx, &block.SerializedHeader, &blockMeta)
		if err != nil {
			return err
		}
	}
	w.BlockConnectEnd(&blockMeta)
	return nil
}

// BlockConnectEnd used to clear some expire data after block connected
func (w *Wallet) BlockConnectEnd(blockMeta *udb.BlockMeta) {
	req := omnilib.Request{
		Method: "omni_onblockconnected",
		Params: []interface{}{blockMeta.Block.Height, blockMeta.Block.Hash.String(), blockMeta.Time.Unix()},
	}
	bytes, err := json.Marshal(req)
	if err == nil {
		omnilib.JsonCmdReqHcToOm(string(bytes))
	}
}

type sideChainBlock struct {
	transactions [][]byte
	headerData   udb.BlockHeaderData
}

// switchToSideChain performs a chain switch, switching the main chain to the
// in-memory side chain.  The old side chain becomes the new main chain.
func (w *Wallet) switchToSideChain(dbtx walletdb.ReadWriteTx) (*MainTipChangedNotification, error) {
	txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)

	sideChain := w.sideChain
	if len(sideChain) == 0 {
		return nil, errors.New("no side chain to switch to")
	}

	sideChainForkHeight := sideChain[0].headerData.SerializedHeader.Height()

	_, tipHeight := w.TxStore.MainChainTip(txmgrNs)
	if tipHeight-sideChainForkHeight+1 < 0 {
		return nil, errors.New("switch to side chain, but tipHeight is smaller than sideChainForkHeight")
	}
	chainTipChanges := &MainTipChangedNotification{
		AttachedBlocks: make([]*chainhash.Hash, len(sideChain)),
		DetachedBlocks: make([]*chainhash.Hash, tipHeight-sideChainForkHeight+1),
		NewHeight:      0, // Must be set by caller before sending
	}

	hashs := make([]chainhash.Hash, 0)
	// Find hashes of removed blocks for notifications.
	for i := tipHeight; i >= sideChainForkHeight; i-- {
		hash, err := w.TxStore.GetMainChainBlockHashForHeight(txmgrNs, i)
		if err != nil {
			return nil, err
		}

		// DetachedBlocks contains block hashes in order of increasing heights.
		chainTipChanges.DetachedBlocks[i-sideChainForkHeight] = &hash

		// For transaction notifications, the blocks are notified in reverse
		// height order.
		w.NtfnServer.notifyDetachedBlock(&hash)
		hashs = append(hashs, hash)
	}

	// Remove blocks on the current main chain that are at or above the
	// height of the block that begins the side chain.
	err := w.RollBack(dbtx, sideChainForkHeight, hashs)
	if err != nil {
		return nil, err
	}

	// Extend the main chain with each sidechain block.
	for i := range sideChain {
		scBlock := &sideChain[i]
		err = w.extendMainChain(dbtx, &scBlock.headerData, scBlock.transactions)
		if err != nil {
			return nil, err
		}

		// Add the block hash to the notification.
		chainTipChanges.AttachedBlocks[i] = &scBlock.headerData.BlockHash
	}

	if sideChain != nil {
		// To avoid skipped blocks, the marker is not advanced if there is a
		// gap between the existing rescan point (main chain fork point of
		// the current marker) and the first block attached in this chain
		// switch.
		r, err := w.rescanPoint(dbtx)
		if err != nil {
			return nil, err
		}
		rHeader, err := w.TxStore.GetBlockHeader(dbtx, r)
		if err != nil {
			return nil, err
		}
		if !(rHeader.Height+1 < uint32(sideChain[0].headerData.SerializedHeader.Height())) {
			marker := sideChain[len(sideChain)-1].headerData.BlockHash
			log.Debugf("Updating processed txs block marker to %v", marker)
			err := w.TxStore.UpdateProcessedTxsBlockMarker(dbtx, &marker)
			if err != nil {
				return nil, err
			}
		}
	}
	return chainTipChanges, nil
}

func (w *Wallet) RollBack(dbtx walletdb.ReadWriteTx, sideChainForkHeight int32, hashs []chainhash.Hash) error {
	addrmgrNs := dbtx.ReadBucket(waddrmgrNamespaceKey)
	txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)

	err := w.TxStore.Rollback(txmgrNs, addrmgrNs, sideChainForkHeight)
	if err != nil {
		return err
	}
	if w.EnableOmni() {
		err = w.RollBackOminiTransaction(uint32(sideChainForkHeight), hashs)
		if err != nil {
			return err
		}
	}
	return nil
}
func copyHeaderSliceToArray(array *udb.RawBlockHeader, slice []byte) error {
	if len(array) != len(udb.RawBlockHeader{}) {
		return errors.New("block header has unexpected size")
	}
	copy(array[:], slice)
	return nil
}

// onBlockConnected is the entry point for processing chain server
// blockconnected notifications.
func (w *Wallet) onBlockConnected(serializedBlockHeader []byte, transactions [][]byte) error {
	var blockHeader wire.BlockHeader
	err := blockHeader.Deserialize(bytes.NewReader(serializedBlockHeader))
	if err != nil {
		return err
	}

	block := udb.BlockHeaderData{BlockHash: blockHeader.BlockHash()}
	err = copyHeaderSliceToArray(&block.SerializedHeader, serializedBlockHeader)
	if err != nil {
		return err
	}

	var chainTipChanges *MainTipChangedNotification

	w.reorganizingLock.Lock()
	reorg, reorgToHash := w.reorganizing, w.reorganizeToHash
	w.reorganizingLock.Unlock()

	w.NtfnServerMutex.Lock()
	if reorg {
		// add to side chain
		scBlock := sideChainBlock{
			transactions: transactions,
			headerData:   block,
		}
		w.sideChain = append(w.sideChain, scBlock)
		log.Infof("Adding block %v (height %v) to sidechain",
			block.BlockHash, block.SerializedHeader.Height())

		if block.BlockHash != reorgToHash {
			// Nothing left to do until the later blocks are
			// received.
			w.NtfnServerMutex.Unlock()
			return nil
		}

		err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
			var err error
			chainTipChanges, err = w.switchToSideChain(dbtx)
			return err
		})
		if err != nil {
			w.NtfnServerMutex.Unlock()
			return err
		}

		w.sideChain = nil
		w.reorganizingLock.Lock()
		w.reorganizing = false
		w.reorganizingLock.Unlock()
		log.Infof("Wallet reorganization to block %v complete", reorgToHash)
	} else {
		err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
			return w.extendMainChain(dbtx, &block, transactions)
		})
		if err != nil {
			w.NtfnServerMutex.Unlock()
			return err
		}
		chainTipChanges = &MainTipChangedNotification{
			AttachedBlocks: []*chainhash.Hash{&block.BlockHash},
			DetachedBlocks: nil,
			NewHeight:      0, // set below
		}
	}

	height := int32(blockHeader.Height)
	chainTipChanges.NewHeight = height

	// Prune all expired transactions and all stake tickets that no longer
	// meet the minimum stake difficulty.
	err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
		//	txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)
		//	return w.TxStore.PruneUnconfirmed(txmgrNs, height, blockHeader.SBits)
		return w.TxStore.PruneUnmined(dbtx, blockHeader.SBits)
	})
	if err != nil {
		log.Errorf("Failed to prune unconfirmed transactions when "+
			"connecting block height %v: %s", height, err.Error())
	}

	w.NtfnServer.notifyMainChainTipChanged(chainTipChanges)
	w.NtfnServer.sendAttachedBlockNotification()
	w.NtfnServerMutex.Unlock()
	if voteVersion(w.chainParams) < blockHeader.StakeVersion {
		log.Warnf("Old vote version detected (v%v), please update your "+
			"wallet to the latest version.", voteVersion(w.chainParams))
	}

	return nil
}

// handleReorganizing handles a blockchain reorganization notification. It
// sets the chain server to indicate that currently the wallet state is in
// reorganizing, and what the final block of the reorganization is by hash.
func (w *Wallet) handleReorganizing(oldHash, newHash *chainhash.Hash, oldHeight, newHeight int64) error {
	w.reorganizingLock.Lock()
	if w.reorganizing {
		reorganizeToHash := w.reorganizeToHash
		w.reorganizingLock.Unlock()

		log.Errorf("Reorg notified for chain tip %v (height %v), but already "+
			"processing a reorg to block %v", newHash, newHeight,
			reorganizeToHash)

		return errors.New("reorganization notified, but reorg already in progress")
	}

	w.reorganizing = true
	w.reorganizeToHash = *newHash
	w.reorganizingLock.Unlock()

	log.Infof("Reorganization detected!")
	log.Infof("Old top block hash: %v", oldHash)
	log.Infof("Old top block height: %v", oldHeight)
	log.Infof("New top block hash: %v", newHash)
	log.Infof("New top block height: %v", newHeight)
	return nil
}

// evaluateStakePoolTicket evaluates a stake pool ticket to see if it's
// acceptable to the stake pool. The ticket must pay out to the stake
// pool cold wallet, and must have a sufficient fee.
func (w *Wallet) evaluateStakePoolTicket(rec *udb.TxRecord,
	blockHeight int32, poolUser hcutil.Address) (bool, error) {
	tx := rec.MsgTx

	// Check the first commitment output (txOuts[1])
	// and ensure that the address found there exists
	// in the list of approved addresses. Also ensure
	// that the fee exists and is of the amount
	// requested by the pool.
	commitmentOut := tx.TxOut[1]
	commitAddr, err := stake.AddrFromSStxPkScrCommitment(
		commitmentOut.PkScript, w.chainParams)
	if err != nil {
		return false, fmt.Errorf("Failed to parse commit out addr: %s",
			err.Error())
	}

	// Extract the fee from the ticket.
	in := hcutil.Amount(0)
	for i := range tx.TxOut {
		if i%2 != 0 {
			commitAmt, err := stake.AmountFromSStxPkScrCommitment(
				tx.TxOut[i].PkScript)
			if err != nil {
				return false, fmt.Errorf("Failed to parse commit "+
					"out amt for commit in vout %v: %s", i, err.Error())
			}
			in += commitAmt
		}
	}
	out := hcutil.Amount(0)
	for i := range tx.TxOut {
		out += hcutil.Amount(tx.TxOut[i].Value)
	}
	fees := in - out

	_, exists := w.stakePoolColdAddrs[commitAddr.EncodeAddress()]
	if exists {
		commitAmt, err := stake.AmountFromSStxPkScrCommitment(
			commitmentOut.PkScript)
		if err != nil {
			return false, fmt.Errorf("failed to parse commit "+
				"out amt: %s", err.Error())
		}

		// Calculate the fee required based on the current
		// height and the required amount from the pool.
		feeNeeded := txrules.StakePoolTicketFee(hcutil.Amount(
			tx.TxOut[0].Value), fees, blockHeight, w.PoolFees(),
			w.ChainParams())
		if commitAmt < feeNeeded {
			log.Warnf("User %s submitted ticket %v which "+
				"has less fees than are required to use this "+
				"stake pool and is being skipped (required: %v"+
				", found %v)", commitAddr.EncodeAddress(),
				tx.TxHash(), feeNeeded, commitAmt)

			// Reject the entire transaction if it didn't
			// pay the pool server fees.
			return false, nil
		}
	} else {
		log.Warnf("Unknown pool commitment address %s for ticket %v",
			commitAddr.EncodeAddress(), tx.TxHash())
		return false, nil
	}

	log.Debugf("Accepted valid stake pool ticket %v committing %v in fees",
		tx.TxHash(), tx.TxOut[0].Value)

	return true, nil
}

func (w *Wallet) processSerializedTransaction(dbtx walletdb.ReadWriteTx, serializedTx []byte, serializedHeader *udb.RawBlockHeader, blockMeta *udb.BlockMeta) error {
	rec, err := udb.NewTxRecord(serializedTx, time.Now())
	if err != nil {
		return err
	}
	/*if len(rec.MsgTx.TxOut) == 3 {
		tempOut := rec.MsgTx.TxOut[2]
		if rec.MsgTx.TxOut[0].PkScript[0] == 200 {
			fmt.Println("test 200")
		}
		if tempOut.PkScript[0] == 106 && len(tempOut.PkScript) == 66 {
			fmt.Println(tempOut)
		}
	}
	*/
	return w.processTransactionRecord(dbtx, rec, serializedHeader, blockMeta)
}

func getPayLoadData(pkScript []byte) (bool, []byte) {
	return txscript.GetPayLoadData(pkScript)
}

// for temp test
func (w *Wallet) RollBackOminiTransaction(height uint32, hashs []chainhash.Hash) error {

	/*
		if len(hashs) == 0 {
			_, h := w.MainChainTip()
			height := height
			for ; height <= uint32(h); height++ {
				//if hashs len = 0, for test
				err := walletdb.Update(w.db, func(tx walletdb.ReadWriteTx) error {
					txmgrNs := tx.ReadWriteBucket(wtxmgrNamespaceKey)
					hash, err := w.TxStore.GetMainChainBlockHashForHeight(txmgrNs, int32(height))
					hashs = append(hashs, hash)
					return err
				})

				if err != nil {
					return err
				}
			}
		}

		strHashs := make([]string, 0)
		for _, hash := range hashs {
			log.Infof("RollBackOminiTransaction: %s", hash.String())
			strHashs = append(strHashs, hash.String())
		}
	*/
	strHashs := make([]string, 0)
	cmd := hcjson.OmniRollBackCmd{
		Height: height,
		Hashs:  &strHashs,
	}

	byteCmd, err := hcjson.MarshalCmd(1, &cmd)
	if err != nil {
		return err
	}

	strRsp := omnilib.JsonCmdReqHcToOm(string(byteCmd))

	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return err
	}
	if response.Error != nil {
		return response.Error
	} else {
		return nil
	}
}

func (w *Wallet) OmniClear() error {
	cmd, err := hcjson.NewCmd("omni_clear")
	if err != nil {
		return err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, cmd)
	if err != nil {
		return err
	}
	//construct omni variables
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
	return nil
}

func (w *Wallet) ProcessOminiTransaction(rec *udb.TxRecord, blockMeta *udb.BlockMeta) error {
	if rec.TxType != stake.TxTypeRegular {
		return nil
	}
	if len(rec.MsgTx.TxIn) == 0 {
		return nil
	}

	if !w.checkValidateOmniTransaction(rec) {
		return nil
	}
	sendIn := rec.MsgTx.TxIn[0]

	if (sendIn.PreviousOutPoint.Hash == chainhash.Hash{}) {
		return nil
	}

	preTxDetail, err := w.chainClient.GetRawTransactionVerbose(&sendIn.PreviousOutPoint.Hash)
	if err != nil {
		fmt.Printf(err.Error())
		return err
	}
	if preTxDetail == nil {
		return fmt.Errorf("local no tx:%v", sendIn.PreviousOutPoint)
	}

	vout := preTxDetail.Vout[sendIn.PreviousOutPoint.Index]
	if len(vout.ScriptPubKey.Addresses) == 0 {
		return errors.New("must assign addresss as sendfrom")
	}
	if len(vout.ScriptPubKey.Addresses) > 1 {
		return errors.New("multiaddress not support")
	}
	sendor := vout.ScriptPubKey.Addresses[0] //多签未考虑
	var toAddress string
	index := int(0)
	isSetMultyNull := false
	isSetToAddress := false
	var payLoad []byte

	for i, txOut := range rec.MsgTx.TxOut {
		ok, payLoad2 := getPayLoadData(txOut.PkScript)
		if ok {
			//nulldata
			if !isSetMultyNull {
				payLoad = payLoad2
				index = i
				isSetMultyNull = true
			} else {
				return errors.New("not allow more than one nulldata script in omini transaction")
			}
		} else {
			if !isSetToAddress {
				_, pubkeyAddrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, txOut.PkScript, w.ChainParams())
				if err != nil {
					return err
				}
				if len(pubkeyAddrs) == 0 || txOut.Value == 0 {
					continue
				}
				if pubkeyAddrs[0].String() != w.chainParams.OmniMoneyReceive {
					toAddress = pubkeyAddrs[0].String() //多签未考虑
					isSetToAddress = true
				}
			}
		}
	}

	if len(payLoad) > 0 {
		//todo move the height restrict code from c++  here
		if string(payLoad) == "payment" {
			err = w.omniTXExodusFundraiser(rec, sendor, blockMeta) // when you send to exodus address and return omni
			if err != nil {
				return err
			}

			err = w.omniProcessPayment(rec, sendor, blockMeta)
			if err != nil {
				return err
			}
		} else {
			fee, err := getFee(w, rec)
			if err != nil {
				return err
			}
			params := []interface{}{
				sendor,
				toAddress,
				rec.Hash.String(),
				blockMeta.Hash.String(),
				int64(blockMeta.Height),
				int64(index),
				hex.EncodeToString(payLoad),
				fee,
				blockMeta.Time.Unix(),
			}

			cmd, err := hcjson.NewCmd("omni_processtx", params...)
			if err != nil {
				return err
			}
			marshalledJSON, err := hcjson.MarshalCmd(1, cmd)
			if err != nil {
				return err
			}
			//construct omni variables
			omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
		}
	}
	return nil
}

func getFee(w *Wallet, rec *udb.TxRecord) (int64, error) {
	amountIn := int64(0)
	amountOut := int64(0)
	for _, in := range rec.MsgTx.TxIn {
		preTxDetail, err := w.chainClient.GetRawTransactionVerbose(&in.PreviousOutPoint.Hash)
		if err != nil {
			return 0, err
		}
		val, _ := hcutil.NewAmount(preTxDetail.Vout[in.PreviousOutPoint.Index].Value)
		amountIn += int64(val)
	}
	for _, out := range rec.MsgTx.TxOut {
		amountOut += out.Value
	}

	fee := amountIn - amountOut
	return fee, nil
}
func (w *Wallet) processTransactionRecord(dbtx walletdb.ReadWriteTx, rec *udb.TxRecord, serializedHeader *udb.RawBlockHeader, blockMeta *udb.BlockMeta) error {

	addrmgrNs := dbtx.ReadWriteBucket(waddrmgrNamespaceKey)
	stakemgrNs := dbtx.ReadWriteBucket(wstakemgrNamespaceKey)
	txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)

	height := int32(-1)
	if serializedHeader != nil {
		height = serializedHeader.Height()
	}

	if w.EnableOmni() && serializedHeader != nil {
		err := w.ProcessOminiTransaction(rec, blockMeta)
		if err != nil {
			return err
		}
	}

	isMineTx, err := w.IsReleventTransaction(dbtx, rec, blockMeta)
	if err != nil {
		return err
	}
	if !isMineTx {
		return nil
	}
	// At the moment all notified transactions are assumed to actually be
	// relevant.  This assumption will not hold true when SPV support is
	// added, but until then, simply insert the transaction because there
	// should either be one or more relevant inputs or outputs.
	if serializedHeader == nil {
		err = w.TxStore.InsertMemPoolTx(txmgrNs, rec)
		if apperrors.IsError(err, apperrors.ErrDuplicate) {
			log.Warnf("Refusing to add unmined transaction %v since same "+
				"transaction already exists mined", &rec.Hash)
			return nil
		}
	} else {
		err = w.TxStore.InsertMinedTx(txmgrNs, addrmgrNs, rec, &blockMeta.Hash)
		if err != nil {
			return err
		}
	}

	// Handle incoming SStx; store them in the stake manager if we own
	// the OP_SSTX tagged out, except if we're operating as a stake pool
	// server. In that case, additionally consider the first commitment
	// output as well.
	isAi, _ := stake.IsAiSStx(&rec.MsgTx)
	if is, _ := stake.IsSStx(&rec.MsgTx); is || isAi{
		// Errors don't matter here.  If addrs is nil, the range below
		// does nothing.
		txOut := rec.MsgTx.TxOut[0]
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(txOut.Version,
			txOut.PkScript, w.chainParams)
		insert := false
		for _, addr := range addrs {
			if !w.Manager.ExistsHash160(addrmgrNs, addr.Hash160()[:]) {
				continue
			}
			// We own the voting output pubkey or script and we're
			// not operating as a stake pool, so simply insert this
			// ticket now.
			if !w.stakePoolEnabled {
				insert = true
				break
			}

			// We are operating as a stake pool. The below
			// function will ONLY add the ticket into the
			// stake pool if it has been found within a
			// block.
			if serializedHeader == nil {
				break
			}

			valid, errEval := w.evaluateStakePoolTicket(rec, height,
				addr)
			if valid {
				// Be sure to insert this into the user's stake
				// pool entry into the stake manager.
				poolTicket := &udb.PoolTicket{
					Ticket:       rec.Hash,
					HeightTicket: uint32(height),
					Status:       udb.TSImmatureOrLive,
				}
				err := w.StakeMgr.UpdateStakePoolUserTickets(
					stakemgrNs, addr, poolTicket)
				if err != nil {
					log.Warnf("Failed to insert stake pool "+
						"user ticket: %v", err)
				}
				log.Debugf("Inserted stake pool ticket %v for user %v "+
					"into the stake store database", &rec.Hash, addr)

				insert = true
				break
			}

			// Log errors if there were any. At this point the ticket
			// must be invalid, so insert it into the list of invalid
			// user tickets.
			if errEval != nil {
				log.Warnf("Ticket %v failed ticket evaluation for "+
					"the stake pool: %s", &rec.Hash, errEval)
			}
			err := w.StakeMgr.UpdateStakePoolUserInvalTickets(
				stakemgrNs, addr, &rec.Hash)
			if err != nil {
				log.Warnf("Failed to update pool user %v with "+
					"invalid ticket %v", addr.EncodeAddress(),
					rec.Hash)
			}
		}

		if insert {
			err := w.StakeMgr.InsertSStx(stakemgrNs, hcutil.NewTx(&rec.MsgTx))
			if err != nil {
				log.Errorf("Failed to insert SStx %v"+
					"into the stake store.", &rec.Hash)
			}
		}
	}

	// Handle incoming votes.  Save a stake manager record for them if we own
	// the ticket used to purchase them.
	if isVote(&rec.MsgTx) && serializedHeader != nil {
		ticketHash := &rec.MsgTx.TxIn[1].PreviousOutPoint.Hash
		if w.TxStore.OwnTicket(dbtx, ticketHash) || w.StakeMgr.OwnTicket(ticketHash) {
			err := w.StakeMgr.InsertSSGen(stakemgrNs, &blockMeta.Block.Hash,
				int64(height), &rec.Hash, stake.SSGenVoteBits(&rec.MsgTx),
				ticketHash)
			if err != nil {
				return err
			}
		}

		// If we're running as a stake pool, insert
		// the stake pool user ticket update too.
		if w.stakePoolEnabled {
			txInHeight := rec.MsgTx.TxIn[1].BlockHeight
			poolTicket := &udb.PoolTicket{
				Ticket:       *ticketHash,
				HeightTicket: txInHeight,
				Status:       udb.TSVoted,
				SpentBy:      rec.Hash,
				HeightSpent:  uint32(height),
			}

			poolUser, err := w.StakeMgr.SStxAddress(stakemgrNs, ticketHash)
			if err != nil {
				log.Warnf("Failed to fetch stake pool user for "+
					"ticket %v (voted ticket): %v", ticketHash, err)
			} else {
				err = w.StakeMgr.UpdateStakePoolUserTickets(
					stakemgrNs, poolUser, poolTicket)
				if err != nil {
					log.Warnf("Failed to update stake pool ticket for "+
						"stake pool user %s after voting",
						poolUser.EncodeAddress())
				} else {
					log.Debugf("Updated voted stake pool ticket %v "+
						"for user %v into the stake store database ("+
						"vote hash: %v)", ticketHash, poolUser, &rec.Hash)
				}
			}
		}
	}

	// Handle incoming revocations.  Store a stake manager record for them if we
	// own the ticket used to purchase them.
	if isRevocation(&rec.MsgTx) && serializedHeader != nil {
		txInHash := &rec.MsgTx.TxIn[0].PreviousOutPoint.Hash

		if w.TxStore.OwnTicket(dbtx, txInHash) || w.StakeMgr.OwnTicket(txInHash) {
			err := w.StakeMgr.StoreRevocationInfo(dbtx, txInHash, &rec.Hash,
				&blockMeta.Hash, height)
			if err != nil {
				return err
			}
		}

		// If we're running as a stake pool, insert
		// the stake pool user ticket update too.
		if w.stakePoolEnabled {
			txInHeight := rec.MsgTx.TxIn[0].BlockHeight
			poolTicket := &udb.PoolTicket{
				Ticket:       *txInHash,
				HeightTicket: txInHeight,
				Status:       udb.TSMissed,
				SpentBy:      rec.Hash,
				HeightSpent:  uint32(height),
			}

			poolUser, err := w.StakeMgr.SStxAddress(stakemgrNs, txInHash)
			if err != nil {
				log.Warnf("failed to fetch stake pool user for "+
					"ticket %v (missed ticket)", txInHash)
			} else {
				err = w.StakeMgr.UpdateStakePoolUserTickets(
					stakemgrNs, poolUser, poolTicket)
				if err != nil {
					log.Warnf("failed to update stake pool ticket for "+
						"stake pool user %s after revoking",
						poolUser.EncodeAddress())
				} else {
					log.Debugf("Updated missed stake pool ticket %v "+
						"for user %v into the stake store database ("+
						"revocation hash: %v)", txInHash, poolUser, &rec.Hash)
				}
			}
		}
	}

	// Handle input scripts that contain P2PKs that we care about.
	for i, input := range rec.MsgTx.TxIn {
		if txscript.IsMultisigSigScript(input.SignatureScript) {
			rs, err := txscript.MultisigRedeemScriptFromScriptSig(input.SignatureScript)
			if err != nil {
				return err
			}

			class, addrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, rs, w.chainParams)
			if err != nil {
				// Non-standard outputs are skipped.
				continue
			}
			if class != txscript.MultiSigTy {
				// This should never happen, but be paranoid.
				continue
			}

			isRelevant := false
			for _, addr := range addrs {
				ma, err := w.Manager.Address(addrmgrNs, addr)
				if err == nil {
					isRelevant = true
					err = w.markUsedAddress(dbtx, ma)
					if err != nil {
						return err
					}
					log.Debugf("Marked address %v used", addr)
				} else {
					// Missing addresses are skipped.  Other errors should
					// be propagated.
					if !apperrors.IsError(err, apperrors.ErrAddressNotFound) {
						return err
					}
				}
			}

			// Add the script to the script databases.
			// TODO Markused script address? cj
			if isRelevant {
				err = w.TxStore.InsertTxScript(txmgrNs, rs)
				if err != nil {
					return err
				}
				mscriptaddr, err := w.Manager.ImportScript(addrmgrNs, rs)
				if err != nil {
					switch {
					// Don't care if it's already there.
					case apperrors.IsError(err, apperrors.ErrDuplicateAddress):
					case apperrors.IsError(err, apperrors.ErrLocked):
						log.Warnf("failed to attempt script importation "+
							"of incoming tx script %x because addrmgr "+
							"was locked", rs)
					default:
						return err
					}
				} else {
					chainClient := w.ChainClient()
					if chainClient != nil {
						err := chainClient.LoadTxFilter(false,
							[]hcutil.Address{mscriptaddr.Address()}, nil)
						if err != nil {
							return err
						}
					}
				}
			}

			// If we're spending a multisig outpoint we know about,
			// update the outpoint. Inefficient because you deserialize
			// the entire multisig output info. Consider a specific
			// exists function in udb. The error here is skipped
			// because the absence of an multisignature output for
			// some script can not always be considered an error. For
			// example, the wallet might be rescanning as called from
			// the above function and so does not have the output
			// included yet.
			mso, err := w.TxStore.GetMultisigOutput(txmgrNs, &input.PreviousOutPoint)
			if mso != nil && err == nil {
				err = w.TxStore.SpendMultisigOut(txmgrNs, &input.PreviousOutPoint,
					rec.Hash,
					uint32(i))
				if err != nil {
					return err
				}
			}
		}
	}

	// Check every output to determine whether it is controlled by a wallet
	// key.  If so, mark the output as a credit.
	for i, output := range rec.MsgTx.TxOut {
		// Ignore unspendable outputs.
		if output.Value == 0 {
			continue
		}

		class, addrs, _, err := txscript.ExtractPkScriptAddrs(output.Version, output.PkScript, w.chainParams)
		if err != nil {
			// Non-standard outputs are skipped.
			continue
		}
		isStakeType := class == txscript.StakeSubmissionTy ||
			class == txscript.StakeSubChangeTy ||
			class == txscript.StakeGenTy ||
			class == txscript.StakeRevocationTy||
			class == txscript.AiStakeSubmissionTy ||
			class == txscript.AiStakeSubChangeTy ||
			class == txscript.AiStakeGenTy ||
			class == txscript.AiStakeRevocationTy
		if isStakeType {
			class, err = txscript.GetStakeOutSubclass(output.PkScript)
			if err != nil {
				log.Errorf("Unknown stake output subclass parse error "+
					"encountered: %v", err)
				continue
			}
		}

		for _, addr := range addrs {
			ma, err := w.Manager.Address(addrmgrNs, addr)
			if err == nil {
				err = w.TxStore.AddCredit(txmgrNs, rec, blockMeta,
					uint32(i), ma.Internal(), ma.Account())
				if err != nil {
					return err
				}
				err = w.markUsedAddress(dbtx, ma)
				if err != nil {
					return err
				}
				log.Debugf("Marked address %v used", addr)
				continue
			}

			// Missing addresses are skipped.  Other errors should
			// be propagated.
			if !apperrors.IsError(err, apperrors.ErrAddressNotFound) {
				return err
			}
		}

		// Handle P2SH addresses that are multisignature scripts
		// with keys that we own.
		if class == txscript.ScriptHashTy {
			var expandedScript []byte
			for _, addr := range addrs {
				// Search both the script store in the tx store
				// and the address manager for the redeem script.
				var err error
				expandedScript, err = w.TxStore.GetTxScript(txmgrNs, addr.ScriptAddress())
				if err != nil {
					return err
				}

				if expandedScript == nil {
					script, done, err := w.Manager.RedeemScript(addrmgrNs, addr)
					if err != nil {
						log.Debugf("failed to find redeemscript for "+"address %v in address manager: %v", addr.EncodeAddress(), err)
						continue
					}
					defer done()
					expandedScript = script
				}
			}

			// Otherwise, extract the actual addresses and
			// see if any belong to us.
			expClass, multisigAddrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, expandedScript, w.chainParams)
			if err != nil {
				return err
			}

			// Skip non-multisig scripts.
			if expClass != txscript.MultiSigTy {
				continue
			}

			for _, maddr := range multisigAddrs {
				_, err := w.Manager.Address(addrmgrNs, maddr)
				// An address we own; handle accordingly.
				if err == nil {
					errStore := w.TxStore.AddMultisigOut(txmgrNs, rec, blockMeta, uint32(i))
					if errStore != nil {
						// This will throw if there are multiple private keys
						// for this multisignature output owned by the wallet,
						// so it's routed to debug.
						log.Debugf("unable to add multisignature output: %v", errStore.Error())
					}
				}
			}
		}
	}

	// Send notification of mined or unmined transaction to any interested
	// clients.
	//
	// TODO: Avoid the extra db hits.
	if serializedHeader == nil {
		details, err := w.TxStore.UniqueTxDetails(txmgrNs, &rec.Hash, nil)
		if err != nil {
			log.Errorf("Cannot query transaction details for notifiation: %v",
				err)
		} else {
			w.NtfnServer.notifyUnminedTransaction(dbtx, details)
		}
	} else {
		details, err := w.TxStore.UniqueTxDetails(txmgrNs, &rec.Hash, &blockMeta.Block)
		if err != nil {
			log.Errorf("Cannot query transaction details for notifiation: %v",
				err)
		} else {
			w.NtfnServer.notifyMinedTransaction(dbtx, details, blockMeta)
		}
	}

	return nil
}

func (w *Wallet) IsReleventTransaction(dbtx walletdb.ReadWriteTx, rec *udb.TxRecord, blockMeta *udb.BlockMeta) (bool, error) {
	addrmgrNs := dbtx.ReadWriteBucket(waddrmgrNamespaceKey)
	// Handle input scripts that contain P2PKs that we care about.
	for _, input := range rec.MsgTx.TxIn {
		if (input.PreviousOutPoint.Hash != chainhash.Hash{}) {
			if txscript.IsMultisigSigScript(input.SignatureScript) {
				rs, err := txscript.MultisigRedeemScriptFromScriptSig(input.SignatureScript)
				if err != nil {
					return false, err
				}

				class, addrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, rs, w.chainParams)
				if err != nil {
					// Non-standard outputs are skipped.
					continue
				}
				if class != txscript.MultiSigTy {
					// This should never happen, but be paranoid.
					continue
				}
				for _, addr := range addrs {
					_, err := w.Manager.Address(addrmgrNs, addr)
					if err == nil {
						return true, nil
					} else {
						// Missing addresses are skipped.  Other errors should
						// be propagated.
						if !apperrors.IsError(err, apperrors.ErrAddressNotFound) {
							return false, err
						}
					}
				}
			} else {
				addr, err := txscript.AddressFromScriptSig(input.SignatureScript, w.chainParams)
				if err != nil {
					return false, err
				}
				_, err = w.Manager.Address(addrmgrNs, addr)
				if err == nil {
					return true, nil
				} else {
					// Missing addresses are skipped.  Other errors should
					// be propagated.
					if !apperrors.IsError(err, apperrors.ErrAddressNotFound) {
						return false, err
					}
				}
			}
		}
	}

	// Check every output to determine whether it is controlled by a wallet
	// key.  If so, mark the output as a credit.
	for _, output := range rec.MsgTx.TxOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(output.Version,
			output.PkScript, w.chainParams)
		if err != nil {
			// Non-standard outputs are skipped.
			continue
		}

		for _, addr := range addrs {
			_, err := w.Manager.Address(addrmgrNs, addr)
			if err == nil {
				return true, nil
			}

			// Missing addresses are skipped.  Other errors should
			// be propagated.
			if !apperrors.IsError(err, apperrors.ErrAddressNotFound) {
				return false, err
			}
		}
	}
	return false, nil
}

func (w *Wallet) handleChainVotingNotifications(chainClient *chain.RPCClient) {
	for n := range chainClient.NotificationsVoting() {
		var err error
		strErrType := ""

		switch n := n.(type) {
		case chain.WinningTickets:
			err = w.handleWinningTickets(n.BlockHash, int32(n.BlockHeight), n.Tickets)
			strErrType = "WinningTickets"
		default:
			err = fmt.Errorf("voting handler received unknown ntfn type")
		}
		if err != nil {
			log.Errorf("Cannot handle chain server voting "+
				"notification %v: %v", strErrType, err)
		}
	}
	w.wg.Done()
}

// selectOwnedTickets returns a slice of tickets hashes from the tickets
// argument that are owned by the wallet.
//
// Because votes must be created for tickets tracked by both the transaction
// manager and the stake manager, this function checks both.
func selectOwnedTickets(w *Wallet, dbtx walletdb.ReadTx, tickets []*chainhash.Hash) []*chainhash.Hash {
	var owned []*chainhash.Hash
	for _, ticketHash := range tickets {
		if w.TxStore.OwnTicket(dbtx, ticketHash) || w.StakeMgr.OwnTicket(ticketHash) {
			owned = append(owned, ticketHash)
		}
	}
	return owned
}

func(w *Wallet) handleInstantTxVote(instantTxVoteHash *chainhash.Hash, instantTxHash *chainhash.Hash, tickeHash *chainhash.Hash, vote bool, sig []byte) {
	log.Debug("handleInstanttxvote")
}


func (w *Wallet) handleNewInstantTx(instantTxBytes []byte, tickets []*chainhash.Hash,resend bool) {

	msgInstantTx:=wire.NewMsgInstantTx()
	msgInstantTx.FromBytes(instantTxBytes)

	var ticketHashes []*chainhash.Hash
	err := walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
		txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)

		// Only consider tickets owned by this wallet.
		ticketHashes = selectOwnedTickets(w, dbtx, tickets)
		if len(ticketHashes) == 0 {
			return nil
		}

		//deal with resend
		if resend{
			msgTx:=msgInstantTx.MsgTx
			//send to normal channel
			w.chainClient.SendRawTransaction(&msgTx,w.AllowHighFees)

			return nil
		}

		//deal with sign
		for _, ticketHash := range ticketHashes {
			ticketPurchase, err := w.TxStore.Tx(txmgrNs, ticketHash)
			if err != nil || ticketPurchase == nil {
				ticketPurchase, err = w.StakeMgr.TicketPurchase(dbtx, ticketHash)
			}
			if err != nil {
				log.Errorf("Failed to read ticket purchase transaction for "+
					"instant ticket %v: %v", ticketHash, err)
				continue
			}

			out := ticketPurchase.TxOut[0]
			//find addr associated with the ticket
			_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.Version,
				out.PkScript, w.chainParams)

			if err != nil {
				log.Errorf("Failed to extract addrs for "+
					"instant ticket %v: %v", ticketHash, err)
				continue
			}

			//instanttxvote
			pk,err:=w.PubKeyForAddress(addrs[0])
			if err!=nil{
				log.Errorf("Failed to extract publick for "+
					"instant ticket %v: %v", ticketHash, err)
				continue
			}

			instantTxVote := wire.NewMsgInstantTxVote()
			instantTxVote.Vote=true
			instantTxVote.TicketHash=*ticketHash
			instantTxVote.InstantTxHash =msgInstantTx.TxHash()
			instantTxVote.PubKey=pk.SerializeCompressed()

			signMsg:=instantTxVote.InstantTxHash.String()+instantTxVote.TicketHash.String()


			//sign msg
			sig,err:=w.SignMessage(signMsg,addrs[0])

			instantTxVote.Sig=sig

			w.chainClient.SendInstantTxVote(instantTxVote)
		}
		return nil
	})



	if resend{
		//update confirm map
		go func() {
			copy:=*msgInstantTx
			w.AiTxConfirms[msgInstantTx.TxHash()]=&copy
		}()

	}


	if err != nil {
		log.Errorf("db View failed handle instant tx: %v", err)
	}

}

// handleWinningTickets receives a list of hashes and some block information
// and submits it to the wstakemgr to handle SSGen production.
func (w *Wallet) handleWinningTickets(blockHash *chainhash.Hash, blockHeight int32, winningTicketHashes []*chainhash.Hash) error {

	if !w.votingEnabled || blockHeight < int32(w.chainParams.StakeValidationHeight)-1 {
		return nil
	}

	chainClient, err := w.requireChainClient()
	if err != nil {
		return err
	}

	// TODO The behavior of this is not quite right if tons of blocks
	// are coming in quickly, because the transaction store will end up
	// out of sync with the voting channel here. This should probably
	// be fixed somehow, but this should be stable for networks that
	// are voting at normal block speeds.

	var ticketHashes []*chainhash.Hash
	var votes []*wire.MsgTx
	winning := false
	voteBits := w.VoteBits()
	err = walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
		txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)

		// Only consider tickets owned by this wallet.
		ticketHashes = selectOwnedTickets(w, dbtx, winningTicketHashes)
		if len(ticketHashes) == 0 {
			return nil
		}

		votes = make([]*wire.MsgTx, len(ticketHashes))

		addrmgrNs := dbtx.ReadBucket(waddrmgrNamespaceKey)
		for i, ticketHash := range ticketHashes {
			ticketPurchase, err := w.TxStore.Tx(txmgrNs, ticketHash)
			if err != nil || ticketPurchase == nil {
				ticketPurchase, err = w.StakeMgr.TicketPurchase(dbtx, ticketHash)
			}
			if err != nil {
				log.Errorf("Failed to read ticket purchase transaction for "+
					"owned winning ticket %v: %v", ticketHash, err)
				continue
			}

			// Don't create votes when this wallet doesn't have voting
			// authority.
			owned, err := w.hasVotingAuthority(addrmgrNs, ticketPurchase)
			if err != nil {
				return err
			}
			if !owned {
				continue
			}

			vote, err := createUnsignedVote(ticketHash, ticketPurchase,
				blockHeight, blockHash, voteBits, w.subsidyCache, w.chainParams)
			if err != nil {
				log.Errorf("Failed to create vote transaction for ticket "+
					"hash %v: %v", ticketHash, err)
				continue
			}
			err = w.signVote(addrmgrNs, ticketPurchase, vote)
			if err != nil {
				log.Errorf("Failed to sign vote for ticket hash %v: %v",
					ticketHash, err)
				continue
			}
			isAiSSGEN, _ := stake.IsAiSSGen(vote)
			if isSSGEN, _ := stake.IsSSGen(vote); !isSSGEN && !isAiSSGEN{
				log.Errorf("not a correct SSGEN format")
				continue
			}
			votes[i] = vote
			winning = true
		}
		return nil
	})
	if err != nil {
		log.Errorf("View failed: %v", err)
	}

	for i, vote := range votes {
		go func(i int, vote *wire.MsgTx) {
			if vote == nil {
				return
			}
			txRec, err := udb.NewTxRecordFromMsgTx(vote, time.Now())
			if err != nil {
				log.Errorf("Failed to create transaction record for vote %v: %v",
					ticketHashes[i], err)
				return
			}
			voteHash := &txRec.Hash
			err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
				err := w.processTransactionRecord(dbtx, txRec, nil, nil)
				if err != nil {
					return err
				}
				err = w.StakeMgr.StoreVoteInfo(dbtx, ticketHashes[i], voteHash,
					blockHash, blockHeight, voteBits)
				if err != nil {
					return err
				}

				_, err = chainClient.SendRawTransaction(vote, true)
				return err
			})
			if err != nil {
				log.Errorf("Failed to send vote for ticket hash %v: %v",
					ticketHashes[i], err)
				return
			}
			log.Infof("Voted on block %v (height %v) using ticket %v "+
				"(vote hash: %v bits: %v)", blockHash, blockHeight,
				ticketHashes[i], voteHash, voteBits.Bits)
		}(i, vote)
	}

	log.Error("winning", winning)

	if winning {
		go func() {

			txMsgR, err := chainClient.FetchPendingTxLock(10)
			if err != nil {
				return
			}

			log.Error("fetchPending", txMsgR)
			for _, txMsgBytes := range txMsgR.MsgTx {
				msgtx := wire.MsgTx{}
				err := msgtx.FromBytes(txMsgBytes)
				if err != nil {
					return
				}
				chainClient.SendRawTransaction(&msgtx, true)
			}

		}()
	}

	return nil
}

// handleMissedTickets receives a list of hashes and some block information
// and submits it to the wstakemgr to handle SSRtx production.
func (w *Wallet) handleMissedTickets(blockHash *chainhash.Hash, blockHeight int32,
	missedTicketHashes []*chainhash.Hash) error {

	if blockHeight < int32(w.chainParams.StakeValidationHeight)-1 {
		return nil
	}

	chainClient, err := w.requireChainClient()
	if err != nil {
		return err
	}

	var ticketHashes []*chainhash.Hash
	var revocations []*wire.MsgTx
	relayFee := w.RelayFee()
	err = walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
		txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)

		// Only consider tickets owned by this wallet.
		ticketHashes = selectOwnedTickets(w, dbtx, missedTicketHashes)
		if len(ticketHashes) == 0 {
			return nil
		}

		revocations = make([]*wire.MsgTx, len(ticketHashes))

		addrmgrNs := dbtx.ReadBucket(waddrmgrNamespaceKey)
		for i, ticketHash := range ticketHashes {
			ticketPurchase, err := w.TxStore.Tx(txmgrNs, ticketHash)
			if err != nil || ticketPurchase == nil {
				ticketPurchase, err = w.StakeMgr.TicketPurchase(dbtx, ticketHash)
			}
			if err != nil {
				log.Errorf("Failed to read ticket purchase transaction for "+
					"missed or expired ticket %v: %v", ticketHash, err)
				continue
			}

			// Don't create revocations when this wallet doesn't have voting
			// authority.
			owned, err := w.hasVotingAuthority(addrmgrNs, ticketPurchase)
			if err != nil {
				return err
			}
			if !owned {
				continue
			}

			revocation, err := createUnsignedRevocation(ticketHash, ticketPurchase,
				relayFee)
			if err != nil {
				log.Errorf("Failed to create revocation transaction for ticket "+
					"hash %v: %v", ticketHash, err)
				continue
			}
			err = w.signRevocation(addrmgrNs, ticketPurchase, revocation)
			if err != nil {
				log.Errorf("Failed to sign revocation for ticket hash %v: %v",
					ticketHash, err)
				continue
			}
			_, errAi := stake.IsAiSSRtx(revocation)
			if _, err := stake.IsSSRtx(revocation); err != nil && errAi != nil{
				log.Errorf("Failed to sign revocation for ticket hash %v: %v",
					ticketHash, err)
			}

			revocations[i] = revocation
		}
		return nil
	})
	if err != nil {
		log.Errorf("View failed: %v", err)
	}

	for i, revocation := range revocations {
		if revocation == nil {
			continue
		}
		txRec, err := udb.NewTxRecordFromMsgTx(revocation, time.Now())
		if err != nil {
			log.Errorf("Failed to create transaction record for revocation %v: %v",
				ticketHashes[i], err)
			continue
		}
		revocationHash := &txRec.Hash
		err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
			err := w.processTransactionRecord(dbtx, txRec, nil, nil)
			if err != nil {
				return err
			}
			err = w.StakeMgr.StoreRevocationInfo(dbtx, ticketHashes[i],
				revocationHash, blockHash, blockHeight)
			if err != nil {
				return err
			}
			_, err = chainClient.SendRawTransaction(revocation, true)
			return err
		})
		if err != nil {
			log.Errorf("Failed to send revocation %v for ticket hash %v: %v",
				revocationHash, ticketHashes[i], err)
			continue
		}
		log.Infof("Revoked ticket %v with revocation %v", ticketHashes[i],
			revocationHash)
	}

	return nil
}

func (w *Wallet) omniTXExodusFundraiser(rec *udb.TxRecord, sendor string, blockMeta *udb.BlockMeta) error {
	amount := int64(0)
	for _, out := range rec.MsgTx.TxOut {
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(out.Version, out.PkScript, w.chainParams)
		if len(addrs) == 1 && addrs[0].String() == w.chainParams.OmniMoneyReceive {
			amount = out.Value
			break
		}
	}
	params := []interface{}{
		rec.Hash.String(),
		sendor,
		int(blockMeta.Height),
		amount,
		int32(blockMeta.Time.Unix()),
	}

	cmd, err := hcjson.NewCmd("omni_txexodus_fundraiser", params...)
	if err != nil {
		return err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, cmd)
	if err != nil {
		return err
	}
	//construct omni variables
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
	return nil
}

func (w *Wallet) omniProcessPayment(rec *udb.TxRecord, sendor string, blockMeta *udb.BlockMeta) error {
	//check every output for property payment
	for i, out := range rec.MsgTx.TxOut {
		if out.Value == 0 {
			return nil
		}
		_, pubkeyAddrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, out.PkScript, w.ChainParams())
		if len(pubkeyAddrs) == 0 || out.Value == 0 {
			continue
		}
		if pubkeyAddrs[0].String() == w.chainParams.OmniMoneyReceive {
			continue
		}
		seller := pubkeyAddrs[0].String() //多签未考虑
		params := []interface{}{
			seller,
			sendor,
			rec.Hash.String(),
			out.Value,
			int64(blockMeta.Height),
			int64(i),
		}

		cmd, err := hcjson.NewCmd("omni_processpayment", params...)
		if err != nil {
			return err
		}
		marshalledJSON, err := hcjson.MarshalCmd(1, cmd)
		if err != nil {
			return err
		}
		//construct omni variables
		omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
	}
	return nil
}

func (w *Wallet) checkValidateOmniTransaction(rec *udb.TxRecord) bool {
	hasOpreturn := false
	hasExodusAddress := false
	for _, txOut := range rec.MsgTx.TxOut {
		ok, _ := getPayLoadData(txOut.PkScript)
		if ok {
			hasOpreturn = true
		} else {
			_, pubkeyAddrs, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, txOut.PkScript, w.ChainParams())
			if err != nil {
				return false
			}
			if len(pubkeyAddrs) == 0 || txOut.Value == 0 {
				continue
			}
			if pubkeyAddrs[0].String() == w.chainParams.OmniMoneyReceive {
				hasExodusAddress = true
			}
		}
	}
	return hasExodusAddress || hasOpreturn
}
