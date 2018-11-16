// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2015-2016 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wallet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/HcashOrg/hcd/chaincfg/chainhash"
	"github.com/HcashOrg/hcrpcclient"
	"github.com/HcashOrg/hcwallet/wallet/udb"
	"github.com/HcashOrg/hcwallet/walletdb"
	"github.com/HcashOrg/hcwallet/omnilib"
	"github.com/HcashOrg/hcd/hcjson"
)

const maxBlocksPerRescan = 2000

var indexScanning int  = 0
var isScanning bool  = false
var mutexOnlyOneChan sync.Mutex

func (w *Wallet) IsScanning() bool{
	mutexOnlyOneChan.Lock()
	ret := isScanning
	mutexOnlyOneChan.Unlock()
	return ret
}
// TODO: track whether a rescan is already in progress, and cancel either it or
// this new rescan, keeping the one that still has the most blocks to scan.

// rescan synchronously scans over all blocks on the main chain starting at
// startHash and height up through the recorded main chain tip block.  The
// progress channel, if non-nil, is sent non-error progress notifications with
// the heights the rescan has completed through, starting with the start height.
func (w *Wallet) rescan(chainClient *hcrpcclient.Client, startHash *chainhash.Hash, height int32,
	p chan<- RescanProgress, cancel <-chan struct{}) error {

	blockHashStorage := make([]chainhash.Hash, maxBlocksPerRescan)
	rescanFrom := *startHash
	inclusive := true

	mutexOnlyOneChan.Lock()
	indexScanning++
	index := indexScanning
	isScanning = true
	mutexOnlyOneChan.Unlock()

	defer func() {
		mutexOnlyOneChan.Lock()
		if indexScanning == index{
			isScanning = false
		}
		mutexOnlyOneChan.Unlock()
	}()
	for {
		select {
		case <-cancel:
			return nil
		default:
		}

		mutexOnlyOneChan.Lock()
		if indexScanning != index{
			mutexOnlyOneChan.Unlock()
			return nil
		}
		mutexOnlyOneChan.Unlock()

		var rescanBlocks []chainhash.Hash
		err := walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
			txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)
			var err error
			rescanBlocks, err = w.TxStore.GetMainChainBlockHashes(txmgrNs,
				&rescanFrom, inclusive, blockHashStorage)
			return err
		})
		if err != nil {
			return err
		}
		if len(rescanBlocks) == 0 {
			return nil
		}

		scanningThrough := height + int32(len(rescanBlocks)) - 1
		log.Infof("Rescanning blocks %v-%v...", height,
			scanningThrough)
		rescanResults, err := chainClient.Rescan(rescanBlocks)
		if err != nil {
			return err
		}
		var rawBlockHeader udb.RawBlockHeader
		err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
			txmgrNs := dbtx.ReadWriteBucket(wtxmgrNamespaceKey)
			for _, r := range rescanResults.DiscoveredData {
				blockHash, err := chainhash.NewHashFromStr(r.Hash)
				if err != nil {
					return err
				}
				blockMeta, err := w.TxStore.GetBlockMetaForHash(txmgrNs, blockHash)
				if err != nil {
					return err
				}
				serHeader, err := w.TxStore.GetSerializedBlockHeader(txmgrNs,
					blockHash)
				if err != nil {
					return err
				}
				err = copyHeaderSliceToArray(&rawBlockHeader, serHeader)
				if err != nil {
					return err
				}

				for _, hexTx := range r.Transactions {
					serTx, err := hex.DecodeString(hexTx)
					if err != nil {
						return err
					}
					err = w.processSerializedTransaction(dbtx, serTx, &rawBlockHeader, &blockMeta)
					if err != nil {
						return err
					}
				}
				err = w.TxStore.UpdateProcessedTxsBlockMarker(dbtx, blockHash)
				if err != nil {
					return err
				}

				if w.EnableOmni() {
					w.BlockConnectEnd(&blockMeta)
				}
			}

			return nil
		})
		if err != nil {
			return err
		}
		mutexOnlyOneChan.Lock()
		err = walletdb.Update(w.db, func(dbtx walletdb.ReadWriteTx) error {
			return w.TxStore.UpdateProcessedTxsBlockMarker(dbtx, &rescanBlocks[len(rescanBlocks)-1])
		})
		if err != nil {
			return err
		}
		if p != nil {
			p <- RescanProgress{ScannedThrough: scanningThrough}
		}
		mutexOnlyOneChan.Unlock()
		rescanFrom = rescanBlocks[len(rescanBlocks)-1]
		height += int32(len(rescanBlocks))
		inclusive = false
	}
}

// Rescan starts a rescan of the wallet for all blocks on the main chain
// beginning at startHash.
//
// An error channel is returned for consumers of this API, but it is not
// required to be read.  If the error can not be immediately written to the
// returned channel, the error will be logged and the channel will be closed.
func (w *Wallet) Rescan(chainClient *hcrpcclient.Client, startHash *chainhash.Hash) <-chan error {
	errc := make(chan error)
	go func() (err error) {
		defer func() {
			select {
			case errc <- err:
			default:
				if err != nil {
					log.Errorf("Rescan failed: %v", err)
				}
				close(errc)
			}
		}()

		var startHeight int32
		err = walletdb.View(w.db, func(tx walletdb.ReadTx) error {
			txmgrNs := tx.ReadBucket(wtxmgrNamespaceKey)
			header, err := w.TxStore.GetSerializedBlockHeader(txmgrNs, startHash)
			if err != nil {
				return err
			}
			startHeight = udb.ExtractBlockHeaderHeight(header)
			return nil
		})
		if err != nil {
			return err
		}

		return w.rescan(chainClient, startHash, startHeight, nil, nil)
	}()

	return errc
}

// RescanFromHeight is an alternative to Rescan that takes a block height
// instead of a hash.  See Rescan for more details.
func (w *Wallet) RescanFromHeight(chainClient *hcrpcclient.Client, startHeight int32) <-chan error {
	errc := make(chan error)

	go func() (err error) {
		defer func() {
			select {
			case errc <- err:
			default:
				if err != nil {
					log.Errorf("Rescan failed: %v", err)
				}
				close(errc)
			}
		}()

		if w.EnableOmni() {
			w.RollBackOminiTransaction(uint32(startHeight), nil)

			req := omnilib.Request{
				Method: "omni_getwaterline",
			}
			bytes, err := json.Marshal(req)
			if err != nil {
				return err
			}
			strRsp := omnilib.JsonCmdReqHcToOm(string(bytes))
			var response hcjson.Response
			err = json.Unmarshal([]byte(strRsp), &response)
			if err != nil {
				return err
			}
			if response.Error != nil {
				return fmt.Errorf(response.Error.Message)
			}
			omni_height, err := strconv.Atoi(string(response.Result))
			if(omni_height <= 0){//need scanwallet from 0
				omni_height = int(startHeight)
			}
			startHeight = int32(omni_height)
		}

		var startHash chainhash.Hash
		err = walletdb.View(w.db, func(tx walletdb.ReadTx) error {
			txmgrNs := tx.ReadBucket(wtxmgrNamespaceKey)
			var err error
			startHash, err = w.TxStore.GetMainChainBlockHashForHeight(
				txmgrNs, startHeight)
			return err
		})
		if err != nil {
			return err
		}
		return w.rescan(chainClient, &startHash, startHeight, nil, nil)
	}()

	return errc
}

// RescanProgress records the height the rescan has completed through and any
// errors during processing of the rescan.
type RescanProgress struct {
	Err            error
	ScannedThrough int32
}

// RescanProgressFromHeight rescans for relevant transactions in all blocks in
// the main chain starting at startHeight.  Progress notifications and any
// errors are sent to the channel p.  This function blocks until the rescan
// completes or ends in an error.  p is closed before returning.
func (w *Wallet) RescanProgressFromHeight(chainClient *hcrpcclient.Client, startHeight int32, p chan<- RescanProgress, cancel <-chan struct{}) {
	defer close(p)

	var startHash chainhash.Hash
	err := walletdb.View(w.db, func(tx walletdb.ReadTx) error {
		txmgrNs := tx.ReadBucket(wtxmgrNamespaceKey)
		var err error
		startHash, err = w.TxStore.GetMainChainBlockHashForHeight(
			txmgrNs, startHeight)
		return err
	})
	if err != nil {
		p <- RescanProgress{Err: err}
		return
	}

	err = w.rescan(chainClient, &startHash, startHeight, p, cancel)
	if err != nil {
		p <- RescanProgress{Err: err}
	}
}
