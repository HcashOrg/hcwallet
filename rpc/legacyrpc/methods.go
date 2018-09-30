// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package legacyrpc

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/HcashOrg/hcd/blockchain/stake"
	"github.com/HcashOrg/hcd/chaincfg"
	"github.com/HcashOrg/hcd/chaincfg/chainec"
	"github.com/HcashOrg/hcd/chaincfg/chainhash"
	"github.com/HcashOrg/hcd/crypto/bliss"
	"github.com/HcashOrg/hcd/hcjson"
	"github.com/HcashOrg/hcd/hcutil"
	"github.com/HcashOrg/hcd/hcutil/hdkeychain"
	"github.com/HcashOrg/hcd/txscript"
	"github.com/HcashOrg/hcd/wire"
	"github.com/HcashOrg/hcrpcclient"
	"github.com/HcashOrg/hcwallet/apperrors"
	"github.com/HcashOrg/hcwallet/wallet"
	"github.com/HcashOrg/hcwallet/wallet/txrules"
	"github.com/HcashOrg/hcwallet/wallet/udb"
)

// API version constants
const (
	jsonrpcSemverString = "4.1.0"
	jsonrpcSemverMajor  = 4
	jsonrpcSemverMinor  = 1
	jsonrpcSemverPatch  = 0
)

var (
	rpcHandlers map[string]LegacyRpcHandler
)

// confirms returns the number of confirmations for a transaction in a block at
// height txHeight (or -1 for an unconfirmed tx) given the chain height
// curHeight.
func confirms(txHeight, curHeight int32) int32 {
	switch {
	case txHeight == -1, txHeight > curHeight:
		return 0
	default:
		return curHeight - txHeight + 1
	}
}

// requestHandler is a handler function to handle an unmarshaled and parsed
// request into a marshalable response.  If the error is a *hcjson.RPCError
// or any of the above special error classes, the server will respond with
// the JSON-RPC appropiate error code.  All other errors use the wallet
// catch-all error code, hcjson.ErrRPCWallet.
type requestHandler func(interface{}, *wallet.Wallet) (interface{}, error)

// requestHandlerChain is a requestHandler that also takes a parameter for
type requestHandlerChainRequired func(interface{}, *wallet.Wallet, *hcrpcclient.Client) (interface{}, error)

type LegacyRpcHandler struct {
	handler          requestHandler
	handlerWithChain requestHandlerChainRequired

	// Function variables cannot be compared against anything but nil, so
	// use a boolean to record whether help generation is necessary.  This
	// is used by the tests to ensure that help can be generated for every
	// implemented method.
	//
	// A single map and this bool is here is used rather than several maps
	// for the unimplemented handlers so every method has exactly one
	// handler function.
	noHelp bool
}

func init() {
	rpcHandlers = map[string]LegacyRpcHandler{
		// Reference implementation wallet methods (implemented)
		"accountaddressindex":     {handler: accountAddressIndex},
		"accountsyncaddressindex": {handler: accountSyncAddressIndex},
		"addmultisigaddress":      {handlerWithChain: addMultiSigAddress},
		"addticket":               {handler: addTicket},
		"consolidate":             {handler: consolidate},
		"createmultisig":          {handler: createMultiSig},
		"dumpprivkey":             {handler: dumpPrivKey},
		"generatevote":            {handler: generateVote},
		"getaccount":              {handler: getAccount},
		"getaccountaddress":       {handler: getAccountAddress},
		"getaddressesbyaccount":   {handler: getAddressesByAccount},
		"getbalance":              {handler: getBalance},
		"getbestblockhash":        {handler: getBestBlockHash},
		"getblockcount":           {handler: getBlockCount},
		"getinfo":                 {handlerWithChain: getInfo},
		"getmasterpubkey":         {handler: getMasterPubkey},
		"getmultisigoutinfo":      {handlerWithChain: getMultisigOutInfo},
		"getnewaddress":           {handler: getNewAddress},
		"getrawchangeaddress":     {handler: getRawChangeAddress},
		"getreceivedbyaccount":    {handler: getReceivedByAccount},
		"getreceivedbyaddress":    {handler: getReceivedByAddress},
		"getstakeinfo":            {handlerWithChain: getStakeInfo},
		"getticketfee":            {handler: getTicketFee},
		"gettickets":              {handlerWithChain: getTickets},
		"gettransaction":          {handler: getTransaction},
		"getvotechoices":          {handler: getVoteChoices},
		"getwalletfee":            {handler: getWalletFee},
		"help":                    {handler: helpNoChainRPC, handlerWithChain: helpWithChainRPC},
		"importprivkey":           {handlerWithChain: importPrivKey},
		"importscript":            {handlerWithChain: importScript},
		"keypoolrefill":           {handler: keypoolRefill},
		"listaccounts":            {handler: listAccounts},
		"listlockunspent":         {handler: listLockUnspent},
		"listreceivedbyaccount":   {handler: listReceivedByAccount},
		"listreceivedbyaddress":   {handler: listReceivedByAddress},
		"listsinceblock":          {handlerWithChain: listSinceBlock},
		"listscripts":             {handler: listScripts},
		"listtransactions":        {handler: listTransactions},
		"listunspent":             {handler: listUnspent},
		"lockunspent":             {handler: lockUnspent},
		"purchaseticket":          {handler: purchaseTicket},
		"rescanwallet":            {handlerWithChain: rescanWallet},
		"revoketickets":           {handlerWithChain: revokeTickets},
		"sendfrom":                {handlerWithChain: sendFrom},
		"sendmany":                {handler: sendMany},
		"sendmanyv2":              {handler: sendManyV2},
		"sendtoaddress":           {handler: sendToAddress},
		"getstraightpubkey":       {handlerWithChain: getStraightPubKey},
		"sendtomultisig":          {handlerWithChain: sendToMultiSig},
		"sendtosstx":              {handlerWithChain: sendToSStx},
		"sendtossgen":             {handler: sendToSSGen},
		"sendtossrtx":             {handlerWithChain: sendToSSRtx},
		"setticketfee":            {handler: setTicketFee},
		"settxfee":                {handler: setTxFee},
		"setvotechoice":           {handler: setVoteChoice},
		"signmessage":             {handler: signMessage},
		"signrawtransaction":      {handler: signRawTransactionNoChainRPC, handlerWithChain: signRawTransaction},
		"signrawtransactions":     {handlerWithChain: signRawTransactions},
		"redeemmultisigout":       {handlerWithChain: redeemMultiSigOut},
		"redeemmultisigouts":      {handlerWithChain: redeemMultiSigOuts},
		"stakepooluserinfo":       {handler: stakePoolUserInfo},
		"ticketsforaddress":       {handler: ticketsForAddress},
		"validateaddress":         {handler: validateAddress},
		"verifymessage":           {handler: verifyMessage},
		"version":                 {handler: versionNoChainRPC, handlerWithChain: versionWithChainRPC},
		"walletinfo":              {handlerWithChain: walletInfo},
		"walletlock":              {handler: walletLock},
		"walletpassphrase":        {handler: walletPassphrase},
		"walletpassphrasechange":  {handler: walletPassphraseChange},

		// Reference implementation methods (still unimplemented)
		"backupwallet":         {handler: unimplemented, noHelp: true},
		"getwalletinfo":        {handler: unimplemented, noHelp: true},
		"importwallet":         {handler: unimplemented, noHelp: true},
		"listaddressgroupings": {handler: unimplemented, noHelp: true},

		// Reference methods which can't be implemented by hcwallet due to
		// design decision differences
		"dumpwallet":    {handler: unsupported, noHelp: true},
		"encryptwallet": {handler: unsupported, noHelp: true},
		"move":          {handler: unsupported, noHelp: true},
		"setaccount":    {handler: unsupported, noHelp: true},

		// Extensions to the reference client JSON-RPC API
		"createnewaccount": {handler: createNewAccount},
		"getbestblock":     {handler: getBestBlock},
		// This was an extension but the reference implementation added it as
		// well, but with a different API (no account parameter).  It's listed
		// here because it hasn't been update to use the reference
		// implemenation's API.
		"getunconfirmedbalance":   {handler: getUnconfirmedBalance},
		"listaddresstransactions": {handler: listAddressTransactions},
		"listalltransactions":     {handler: listAllTransactions},
		"renameaccount":           {handler: renameAccount},
		"walletislocked":          {handler: walletIsLocked},
	}

	for k, v := range getOminiMethod() {
		rpcHandlers[k] = v
	}
}

// unimplemented handles an unimplemented RPC request with the
// appropiate error.
func unimplemented(interface{}, *wallet.Wallet) (interface{}, error) {
	return nil, &hcjson.RPCError{
		Code:    hcjson.ErrRPCUnimplemented,
		Message: "Method unimplemented",
	}
}

// unsupported handles a standard bitcoind RPC request which is
// unsupported by hcwallet due to design differences.
func unsupported(interface{}, *wallet.Wallet) (interface{}, error) {
	return nil, &hcjson.RPCError{
		Code:    -1,
		Message: "Request unsupported by hcwallet",
	}
}

// lazyHandler is a closure over a requestHandler or passthrough request with
// the RPC server's wallet and chain server variables as part of the closure
// context.
type lazyHandler func() (interface{}, *hcjson.RPCError)

// lazyApplyHandler looks up the best request handler func for the method,
// returning a closure that will execute it with the (required) wallet and
// (optional) consensus RPC server.  If no handlers are found and the
// chainClient is not nil, the returned handler performs RPC passthrough.
func lazyApplyHandler(request *hcjson.Request, w *wallet.Wallet, chainClient *hcrpcclient.Client) lazyHandler {
	handlerData, ok := rpcHandlers[request.Method]
	if ok && handlerData.handlerWithChain != nil && w != nil && chainClient != nil {
		return func() (interface{}, *hcjson.RPCError) {
			cmd, err := hcjson.UnmarshalCmd(request)
			if err != nil {
				return nil, hcjson.ErrRPCInvalidRequest
			}
			resp, err := handlerData.handlerWithChain(cmd, w, chainClient)
			if err != nil {
				return nil, jsonError(err)
			}
			return resp, nil
		}
	}
	if ok && handlerData.handler != nil && w != nil {
		return func() (interface{}, *hcjson.RPCError) {
			cmd, err := hcjson.UnmarshalCmd(request)
			if err != nil {
				return nil, hcjson.ErrRPCInvalidRequest
			}
			resp, err := handlerData.handler(cmd, w)
			if err != nil {
				return nil, jsonError(err)
			}
			return resp, nil
		}
	}

	// Fallback to RPC passthrough
	return func() (interface{}, *hcjson.RPCError) {
		if chainClient == nil {
			return nil, &hcjson.RPCError{
				Code:    -1,
				Message: "Chain RPC is inactive",
			}
		}
		resp, err := chainClient.RawRequest(request.Method, request.Params)
		if err != nil {
			return nil, jsonError(err)
		}
		return &resp, nil
	}
}

// makeResponse makes the JSON-RPC response struct for the result and error
// returned by a requestHandler.  The returned response is not ready for
// marshaling and sending off to a client, but must be
func makeResponse(id, result interface{}, err error) hcjson.Response {
	idPtr := idPointer(id)
	if err != nil {
		return hcjson.Response{
			ID:    idPtr,
			Error: jsonError(err),
		}
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return hcjson.Response{
			ID: idPtr,
			Error: &hcjson.RPCError{
				Code:    hcjson.ErrRPCInternal.Code,
				Message: "Unexpected error marshalling result",
			},
		}
	}
	return hcjson.Response{
		ID:     idPtr,
		Result: json.RawMessage(resultBytes),
	}
}

// jsonError creates a JSON-RPC error from the Go error.
func jsonError(err error) *hcjson.RPCError {
	if err == nil {
		return nil
	}

	code := hcjson.ErrRPCWallet
	switch e := err.(type) {
	case hcjson.RPCError:
		return &e
	case *hcjson.RPCError:
		return e
	case DeserializationError:
		code = hcjson.ErrRPCDeserialization
	case InvalidParameterError:
		code = hcjson.ErrRPCInvalidParameter
	case ParseError:
		code = hcjson.ErrRPCParse.Code
	case apperrors.E:
		switch e.ErrorCode {
		case apperrors.ErrWrongPassphrase:
			code = hcjson.ErrRPCWalletPassphraseIncorrect
		}
	}
	return &hcjson.RPCError{
		Code:    code,
		Message: err.Error(),
	}
}

// accountAddressIndex returns the next address index for the passed
// account and branch.
func accountAddressIndex(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.AccountAddressIndexCmd)
	account, err := w.AccountNumber(cmd.Account)
	if err != nil {
		return nil, err
	}

	extChild, intChild, err := w.BIP0044BranchNextIndexes(account)
	if err != nil {
		return nil, err
	}
	switch uint32(cmd.Branch) {
	case udb.ExternalBranch:
		return extChild, nil
	case udb.InternalBranch:
		return intChild, nil
	default:
		// The branch may only be internal or external.
		return nil, fmt.Errorf("invalid branch %v", cmd.Branch)
	}
}

// accountSyncAddressIndex synchronizes the address manager and local address
// pool for some account and branch to the passed index. If the current pool
// index is beyond the passed index, an error is returned. If the passed index
// is the same as the current pool index, nothing is returned. If the syncing
// is successful, nothing is returned.
func accountSyncAddressIndex(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.AccountSyncAddressIndexCmd)
	account, err := w.AccountNumber(cmd.Account)
	if err != nil {
		return nil, err
	}

	branch := uint32(cmd.Branch)
	index := uint32(cmd.Index)

	if index >= hdkeychain.HardenedKeyStart {
		return nil, fmt.Errorf("child index %d exceeds the maximum child index "+
			"for an account", index)
	}

	// Additional addresses need to be watched.  Since addresses are derived
	// based on the last used address, this RPC no longer changes the child
	// indexes that new addresses are derived from.
	return nil, w.ExtendWatchedAddresses(account, branch, index)
}

func makeMultiSigScript(w *wallet.Wallet, keys []string,
	nRequired int) ([]byte, error) {
	keysesPrecious := make([]hcutil.Address, len(keys))

	// The address list will made up either of addreseses (pubkey hash), for
	// which we need to look up the keys in wallet, straight pubkeys, or a
	// mixture of the two.
	for i, a := range keys {
		// try to parse as pubkey address
		a, err := decodeAddress(a, w.ChainParams())
		if err != nil {
			return nil, err
		}

		switch addr := a.(type) {
		case *hcutil.AddressSecpPubKey:
			keysesPrecious[i] = addr
		case *hcutil.AddressBlissPubKey:
			keysesPrecious[i] = addr
		default:
			pubKey, err := w.PubKeyForAddress(addr)
			if err != nil {
				return nil, err
			}

			pkType := pubKey.GetType()
			if pkType == chainec.ECTypeSecp256k1 {
				pubKeyAddr, err := hcutil.NewAddressSecpPubKey(pubKey.Serialize(), w.ChainParams())
				if err != nil {
					return nil, err
				}
				keysesPrecious[i] = pubKeyAddr
			} else if pkType == bliss.BSTypeBliss {
				pubKeyAddr, err := hcutil.NewAddressBlissPubKey(pubKey.Serialize(), w.ChainParams())
				if err != nil {
					return nil, err
				}
				keysesPrecious[i] = pubKeyAddr
			} else {
				return nil, fmt.Errorf("address type(%d) err", pkType)
			}
		}
	}

	return txscript.MultiSigScript(keysesPrecious, nRequired)
}

// addMultiSigAddress handles an addmultisigaddress request by adding a
// multisig address to the given wallet.
func addMultiSigAddress(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.AddMultisigAddressCmd)

	// If an account is specified, ensure that is the imported account.
	if cmd.Account != nil && *cmd.Account != udb.ImportedAddrAccountName {
		return nil, &ErrNotImportedAccount
	}

	addrs := make([]hcutil.Address, len(cmd.Keys))
	for i, k := range cmd.Keys {
		addr, err := decodeAddress(k, w.ChainParams())
		if err != nil {
			return nil, ParseError{err}
		}
		addrs[i] = addr
	}

	script, err := w.MakeMultiSigScript(addrs, cmd.NRequired)
	if err != nil {
		return nil, err
	}

	p2shAddr, err := w.ImportP2SHRedeemScript(script)
	if err != nil {
		return nil, err
	}

	err = chainClient.LoadTxFilter(false, []hcutil.Address{p2shAddr}, nil)
	if err != nil {
		return nil, err
	}

	return p2shAddr.EncodeAddress(), nil
}

// addTicket adds a ticket to the stake manager manually.
func addTicket(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.AddTicketCmd)

	rawTx, err := hex.DecodeString(cmd.TicketHex)
	if err != nil {
		return nil, err
	}

	mtx := new(wire.MsgTx)
	err = mtx.FromBytes(rawTx)
	if err != nil {
		return nil, err
	}
	err = w.AddTicket(mtx)

	return nil, err
}

// consolidate handles a consolidate request by returning attempting to compress
// as many inputs as given and then returning the txHash and error.
func consolidate(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ConsolidateCmd)

	account := uint32(udb.DefaultAccountNum)
	var err error
	if cmd.Account != nil {
		account, err = w.AccountNumber(*cmd.Account)
		if err != nil {
			return nil, err
		}
	}

	// Set change address if specified.
	var changeAddr hcutil.Address
	if cmd.Address != nil {
		if *cmd.Address != "" {
			addr, err := decodeAddress(*cmd.Address, w.ChainParams())
			if err != nil {
				return nil, err
			}
			changeAddr = addr
		}
	}

	// TODO In the future this should take the optional account and
	// only consolidate UTXOs found within that account.
	txHash, err := w.Consolidate(cmd.Inputs, account, changeAddr)
	if err != nil {
		return nil, err
	}

	return txHash.String(), nil
}

// createMultiSig handles an createmultisig request by returning a
// multisig address for the given inputs.
func createMultiSig(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.CreateMultisigCmd)

	script, err := makeMultiSigScript(w, cmd.Keys, cmd.NRequired)
	if err != nil {
		return nil, ParseError{err}
	}

	address, err := hcutil.NewAddressScriptHash(script, w.ChainParams())
	if err != nil {
		// above is a valid script, shouldn't happen.
		return nil, err
	}

	return hcjson.CreateMultiSigResult{
		Address:      address.EncodeAddress(),
		RedeemScript: hex.EncodeToString(script),
	}, nil
}

// dumpPrivKey handles a dumpprivkey request with the private key
// for a single address, or an appropiate error if the wallet
// is locked.
func dumpPrivKey(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.DumpPrivKeyCmd)

	addr, err := decodeAddress(cmd.Address, w.ChainParams())
	if err != nil {
		return nil, err
	}

	key, err := w.DumpWIFPrivateKey(addr)
	if apperrors.IsError(err, apperrors.ErrLocked) {
		// Address was found, but the private key isn't
		// accessible.
		return nil, &ErrWalletUnlockNeeded
	}
	return key, err
}

// generateVote handles a generatevote request by constructing a signed
// vote and returning it.
func generateVote(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GenerateVoteCmd)

	blockHash, err := chainhash.NewHashFromStr(cmd.BlockHash)
	if err != nil {
		return nil, err
	}

	ticketHash, err := chainhash.NewHashFromStr(cmd.TicketHash)
	if err != nil {
		return nil, err
	}

	var voteBitsExt []byte
	voteBitsExt, err = hex.DecodeString(cmd.VoteBitsExt)
	if err != nil {
		return nil, err
	}
	voteBits := stake.VoteBits{
		Bits:         cmd.VoteBits,
		ExtendedBits: voteBitsExt,
	}

	ssgentx, err := w.GenerateVoteTx(blockHash, int32(cmd.Height), ticketHash,
		voteBits)
	if err != nil {
		return nil, err
	}

	txHex := ""
	if ssgentx != nil {
		// Serialize the transaction and convert to hex string.
		buf := bytes.NewBuffer(make([]byte, 0, ssgentx.SerializeSize()))
		if err := ssgentx.Serialize(buf); err != nil {
			return nil, err
		}
		txHex = hex.EncodeToString(buf.Bytes())
	}

	resp := &hcjson.GenerateVoteResult{
		Hex: txHex,
	}

	return resp, nil
}

// getAddressesByAccount handles a getaddressesbyaccount request by returning
// all addresses for an account, or an error if the requested account does
// not exist.
func getAddressesByAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetAddressesByAccountCmd)

	account, err := w.AccountNumber(cmd.Account)
	if err != nil {
		return nil, err
	}

	if account == udb.ImportedAddrAccount {
		return w.FetchImortedAccountAddress()
	}
	// Find the next child address indexes for the account.
	endExt, endInt, err := w.BIP0044BranchNextIndexes(account)
	if err != nil {
		return nil, err
	}

	// Nothing to do if we have no addresses.
	if endExt+endInt == 0 {
		return nil, nil
	}

	// Derive the addresses.
	addrsStr := make([]string, endInt+endExt)
	addrsExt, err := w.AccountBranchAddressRange(account, udb.ExternalBranch, 0, endExt)
	if err != nil {
		return nil, err
	}
	for i := range addrsExt {
		addrsStr[i] = addrsExt[i].EncodeAddress()
	}
	addrsInt, err := w.AccountBranchAddressRange(account, udb.InternalBranch, 0, endInt)
	if err != nil {
		return nil, err
	}
	for i := range addrsInt {
		addrsStr[i+int(endExt)] = addrsInt[i].EncodeAddress()
	}

	return addrsStr, nil
}

// getBalance handles a getbalance request by returning the balance for an
// account (wallet), or an error if the requested account does not
// exist.
func getBalance(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetBalanceCmd)

	minConf := int32(*cmd.MinConf)
	if minConf < 0 {
		e := errors.New("minconf must be non-negative")
		return nil, InvalidParameterError{e}
	}

	accountName := "*"
	if cmd.Account != nil {
		accountName = *cmd.Account
	}

	blockHash, _ := w.MainChainTip()
	result := hcjson.GetBalanceResult{
		BlockHash: blockHash.String(),
	}

	if accountName == "*" {
		balances, err := w.CalculateAccountBalances(int32(*cmd.MinConf))
		if err != nil {
			return nil, err
		}

		var (
			totImmatureCoinbase hcutil.Amount
			totImmatureStakegen hcutil.Amount
			totLocked           hcutil.Amount
			totSpendable        hcutil.Amount
			totUnconfirmed      hcutil.Amount
			totVotingAuthority  hcutil.Amount
			cumTot              hcutil.Amount
		)

		for _, bal := range balances {
			accountName, err := w.AccountName(bal.Account)
			if err != nil {
				return nil, err
			}

			totImmatureCoinbase += bal.ImmatureCoinbaseRewards
			totImmatureStakegen += bal.ImmatureStakeGeneration
			totLocked += bal.LockedByTickets
			totSpendable += bal.Spendable
			totUnconfirmed += bal.Unconfirmed
			totVotingAuthority += bal.VotingAuthority
			cumTot += bal.Total

			json := hcjson.GetAccountBalanceResult{
				AccountName:             accountName,
				ImmatureCoinbaseRewards: bal.ImmatureCoinbaseRewards.ToCoin(),
				ImmatureStakeGeneration: bal.ImmatureStakeGeneration.ToCoin(),
				LockedByTickets:         bal.LockedByTickets.ToCoin(),
				Spendable:               bal.Spendable.ToCoin(),
				Total:                   bal.Total.ToCoin(),
				Unconfirmed:             bal.Unconfirmed.ToCoin(),
				VotingAuthority:         bal.VotingAuthority.ToCoin(),
			}
			result.Balances = append(result.Balances, json)
		}

		result.TotalImmatureCoinbaseRewards = totImmatureCoinbase.ToCoin()
		result.TotalImmatureStakeGeneration = totImmatureStakegen.ToCoin()
		result.TotalLockedByTickets = totLocked.ToCoin()
		result.TotalSpendable = totSpendable.ToCoin()
		result.TotalUnconfirmed = totUnconfirmed.ToCoin()
		result.TotalVotingAuthority = totVotingAuthority.ToCoin()
		result.CumulativeTotal = cumTot.ToCoin()
	} else {
		account, err := w.AccountNumber(accountName)
		if err != nil {
			return nil, err
		}

		bal, err := w.CalculateAccountBalance(account, int32(*cmd.MinConf))
		if err != nil {
			return nil, err
		}
		json := hcjson.GetAccountBalanceResult{
			AccountName:             accountName,
			ImmatureCoinbaseRewards: bal.ImmatureCoinbaseRewards.ToCoin(),
			ImmatureStakeGeneration: bal.ImmatureStakeGeneration.ToCoin(),
			LockedByTickets:         bal.LockedByTickets.ToCoin(),
			Spendable:               bal.Spendable.ToCoin(),
			Total:                   bal.Total.ToCoin(),
			Unconfirmed:             bal.Unconfirmed.ToCoin(),
			VotingAuthority:         bal.VotingAuthority.ToCoin(),
		}
		result.Balances = append(result.Balances, json)
	}

	return result, nil
}

// getBestBlock handles a getbestblock request by returning a JSON object
// with the height and hash of the most recently processed block.
func getBestBlock(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	hash, height := w.MainChainTip()
	result := &hcjson.GetBestBlockResult{
		Hash:   hash.String(),
		Height: int64(height),
	}
	return result, nil
}

// getBestBlockHash handles a getbestblockhash request by returning the hash
// of the most recently processed block.
func getBestBlockHash(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	hash, _ := w.MainChainTip()
	return hash.String(), nil
}

// getBlockCount handles a getblockcount request by returning the chain height
// of the most recently processed block.
func getBlockCount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_, height := w.MainChainTip()
	return height, nil
}

// getInfo handles a getinfo request by returning the a structure containing
// information about the current state of hcwallet.
// exist.
func getInfo(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	// Call down to hcd for all of the information in this command known
	// by them.
	info, err := chainClient.GetInfo()
	if err != nil {
		return nil, err
	}

	balances, err := w.CalculateAccountBalances(1)
	if err != nil {
		return nil, err
	}

	var bal hcutil.Amount
	for _, balance := range balances {
		bal += balance.Spendable
	}

	info.WalletVersion = udb.DBVersion
	info.Balance = bal.ToCoin()
	info.KeypoolOldest = time.Now().Unix()
	info.KeypoolSize = 0
	info.PaytxFee = w.RelayFee().ToCoin()
	// We don't set the following since they don't make much sense in the
	// wallet architecture:
	//  - unlocked_until
	//  - errors

	return info, nil
}

func decodeAddress(s string, params *chaincfg.Params) (hcutil.Address, error) {
	// Secp256k1 pubkey as a string, handle differently.
	if len(s) == 66 || len(s) == 130 {
		pubKeyBytes, err := hex.DecodeString(s)
		if err != nil {
			return nil, err
		}
		pubKeyAddr, err := hcutil.NewAddressSecpPubKey(pubKeyBytes,
			params)
		if err != nil {
			return nil, err
		}

		return pubKeyAddr, nil
	}

	if len(s) == 1794 {
		pubKeyBytes, err := hex.DecodeString(s)
		if err != nil {
			return nil, err
		}
		pubKeyAddr, err := hcutil.NewAddressBlissPubKey(pubKeyBytes, params)
		if err != nil {
			return nil, err
		}

		return pubKeyAddr, nil
	}

	addr, err := hcutil.DecodeAddress(s)
	if err != nil {
		msg := fmt.Sprintf("Invalid address %q: decode failed with %#q", s, err)
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCInvalidAddressOrKey,
			Message: msg,
		}
	}
	if !addr.IsForNet(params) {
		msg := fmt.Sprintf("Invalid address %q: not intended for use on %s",
			addr, params.Name)
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCInvalidAddressOrKey,
			Message: msg,
		}
	}
	return addr, nil
}

// getAccount handles a getaccount request by returning the account name
// associated with a single address.
func getAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetAccountCmd)

	addr, err := decodeAddress(cmd.Address, w.ChainParams())
	if err != nil {
		return nil, err
	}

	// Fetch the associated account
	account, err := w.AccountOfAddress(addr)
	if err != nil {
		return nil, &ErrAddressNotInWallet
	}

	acctName, err := w.AccountName(account)
	if err != nil {
		return nil, &ErrAccountNameNotFound
	}
	return acctName, nil
}

// getAccountAddress handles a getaccountaddress by returning the most
// recently-created chained address that has not yet been used (does not yet
// appear in the blockchain, or any tx that has arrived in the hcd mempool).
// If the most recently-requested address has been used, a new address (the
// next chained address in the keypool) is used.  This can fail if the keypool
// runs out (and will return hcjson.ErrRPCWalletKeypoolRanOut if that happens).
func getAccountAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetAccountAddressCmd)

	account, err := w.AccountNumber(cmd.Account)
	if err != nil {
		return nil, err
	}
	addr, err := w.CurrentAddress(account)
	if err != nil {
		return nil, err
	}

	return addr.EncodeAddress(), err
}

// getUnconfirmedBalance handles a getunconfirmedbalance extension request
// by returning the current unconfirmed balance of an account.
func getUnconfirmedBalance(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetUnconfirmedBalanceCmd)

	acctName := "default"
	if cmd.Account != nil {
		acctName = *cmd.Account
	}
	account, err := w.AccountNumber(acctName)
	if err != nil {
		return nil, err
	}
	bals, err := w.CalculateAccountBalance(account, 1)
	if err != nil {
		return nil, err
	}

	return (bals.Total - bals.Spendable).ToCoin(), nil
}

// importPrivKey handles an importprivkey request by parsing
// a WIF-encoded private key and adding it to an account.
func importPrivKey(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.ImportPrivKeyCmd)

	// Ensure that private keys are only imported to the correct account.
	//
	// Yes, Label is the account name.
	if cmd.Label != nil && *cmd.Label != udb.ImportedAddrAccountName {
		return nil, &ErrNotImportedAccount
	}

	wif, err := hcutil.DecodeWIF(cmd.PrivKey)
	if err != nil {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCInvalidAddressOrKey,
			Message: "WIF decode failed: " + err.Error(),
		}
	}
	if !wif.IsForNet(w.ChainParams()) {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCInvalidAddressOrKey,
			Message: "Key is not intended for " + w.ChainParams().Name,
		}
	}

	rescan := true
	if cmd.Rescan != nil {
		rescan = *cmd.Rescan
	}

	scanFrom := int32(0)
	if cmd.ScanFrom != nil {
		scanFrom = int32(*cmd.ScanFrom)
	}

	// Import the private key, handling any errors.
	_, err = w.ImportPrivateKey(wif)
	switch {
	case apperrors.IsError(err, apperrors.ErrDuplicateAddress):
		// Do not return duplicate key errors to the client.
		return nil, nil
	case apperrors.IsError(err, apperrors.ErrLocked):
		return nil, &ErrWalletUnlockNeeded
	}

	if rescan {
		w.RescanFromHeight(chainClient, scanFrom, false)
	}

	return nil, err
}

// importScript imports a redeem script for a P2SH output.
func importScript(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.ImportScriptCmd)
	rs, err := hex.DecodeString(cmd.Hex)
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return nil, fmt.Errorf("passed empty script")
	}

	rescan := true
	if cmd.Rescan != nil {
		rescan = *cmd.Rescan
	}

	scanFrom := 0
	if cmd.ScanFrom != nil {
		scanFrom = *cmd.ScanFrom
	}

	err = w.ImportScript(rs)
	if err != nil {
		return nil, err
	}

	if rescan {
		w.RescanFromHeight(chainClient, int32(scanFrom), false)
	}

	return nil, nil
}

// keypoolRefill handles the keypoolrefill command. Since we handle the keypool
// automatically this does nothing since refilling is never manually required.
func keypoolRefill(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return nil, nil
}

// createNewAccount handles a createnewaccount request by creating and
// returning a new account. If the last account has no transaction history
// as per BIP 0044 a new account cannot be created so an error will be returned.
func createNewAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return nil, fmt.Errorf("not support this function now")
}

// renameAccount handles a renameaccount request by renaming an account.
// If the account does not exist an appropiate error will be returned.
func renameAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.RenameAccountCmd)

	// The wildcard * is reserved by the rpc server with the special meaning
	// of "all accounts", so disallow naming accounts to this string.
	if cmd.NewAccount == "*" {
		return nil, &ErrReservedAccountName
	}

	// Check that given account exists
	account, err := w.AccountNumber(cmd.OldAccount)
	if err != nil {
		return nil, err
	}
	return nil, w.RenameAccount(account, cmd.NewAccount)
}

// getMultisigOutInfo displays information about a given multisignature
// output.
func getMultisigOutInfo(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.GetMultisigOutInfoCmd)

	hash, err := chainhash.NewHashFromStr(cmd.Hash)
	if err != nil {
		return nil, err
	}

	// Multisig outs are always in TxTreeRegular.
	op := &wire.OutPoint{
		Hash:  *hash,
		Index: cmd.Index,
		Tree:  wire.TxTreeRegular,
	}

	p2shOutput, err := w.FetchP2SHMultiSigOutput(op)
	if err != nil {
		return nil, err
	}

	// Get the list of pubkeys required to sign.
	var pubkeys []string
	_, pubkeyAddrs, _, err := txscript.ExtractPkScriptAddrs(
		txscript.DefaultScriptVersion, p2shOutput.RedeemScript,
		w.ChainParams())
	if err != nil {
		return nil, err
	}
	for _, pka := range pubkeyAddrs {
		pubkeys = append(pubkeys, hex.EncodeToString(pka.ScriptAddress()))
	}

	result := &hcjson.GetMultisigOutInfoResult{
		Address:      p2shOutput.P2SHAddress.EncodeAddress(),
		RedeemScript: hex.EncodeToString(p2shOutput.RedeemScript),
		M:            p2shOutput.M,
		N:            p2shOutput.N,
		Pubkeys:      pubkeys,
		TxHash:       p2shOutput.OutPoint.Hash.String(),
		Amount:       p2shOutput.OutputAmount.ToCoin(),
	}
	if !p2shOutput.ContainingBlock.None() {
		result.BlockHeight = uint32(p2shOutput.ContainingBlock.Height)
		result.BlockHash = p2shOutput.ContainingBlock.Hash.String()
	}
	if p2shOutput.Redeemer != nil {
		result.Spent = true
		result.SpentBy = p2shOutput.Redeemer.TxHash.String()
		result.SpentByIndex = p2shOutput.Redeemer.InputIndex
	}
	return result, nil
}

// getNewAddress handles a getnewaddress request by returning a new
// address for an account.  If the account does not exist an appropiate
// error is returned.
// TODO: Follow BIP 0044 and warn if number of unused addresses exceeds
// the gap limit.
func getNewAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetNewAddressCmd)

	var callOpts []wallet.NextAddressCallOption
	if cmd.GapPolicy != nil {
		switch *cmd.GapPolicy {
		case "":
		case "error":
			callOpts = append(callOpts, wallet.WithGapPolicyError())
		case "ignore":
			callOpts = append(callOpts, wallet.WithGapPolicyIgnore())
		case "wrap":
			callOpts = append(callOpts, wallet.WithGapPolicyWrap())
		default:
			return nil, &hcjson.RPCError{
				Code:    hcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Unknown gap policy '%s'", *cmd.GapPolicy),
			}
		}
	}

	acctName := "default"
	if cmd.Account != nil {
		acctName = *cmd.Account
	}
	account, err := w.AccountNumber(acctName)
	if err != nil {
		return nil, err
	}

	addr, err := w.NewExternalAddress(account, callOpts...)
	if err != nil {
		return nil, err
	}
	return addr.EncodeAddress(), nil
}

// getRawChangeAddress handles a getrawchangeaddress request by creating
// and returning a new change address for an account.
//
// Note: bitcoind allows specifying the account as an optional parameter,
// but ignores the parameter.
func getRawChangeAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetRawChangeAddressCmd)

	acctName := "default"
	if cmd.Account != nil {
		acctName = *cmd.Account
	}
	account, err := w.AccountNumber(acctName)
	if err != nil {
		return nil, err
	}

	addr, err := w.NewChangeAddress(account)
	if err != nil {
		return nil, err
	}

	// Return the new payment address string.
	return addr.EncodeAddress(), nil
}

// getReceivedByAccount handles a getreceivedbyaccount request by returning
// the total amount received by addresses of an account.
func getReceivedByAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetReceivedByAccountCmd)

	account, err := w.AccountNumber(cmd.Account)
	if err != nil {
		return nil, err
	}

	// TODO: This is more inefficient that it could be, but the entire
	// algorithm is already dominated by reading every transaction in the
	// wallet's history.
	results, err := w.TotalReceivedForAccounts(int32(*cmd.MinConf))
	if err != nil {
		return nil, err
	}
	acctIndex := int(account)
	if account == udb.ImportedAddrAccount {
		acctIndex = len(results) - 1
	}
	return results[acctIndex].TotalReceived.ToCoin(), nil
}

// getReceivedByAddress handles a getreceivedbyaddress request by returning
// the total amount received by a single address.
func getReceivedByAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetReceivedByAddressCmd)

	addr, err := decodeAddress(cmd.Address, w.ChainParams())
	if err != nil {
		return nil, err
	}
	total, err := w.TotalReceivedForAddr(addr, int32(*cmd.MinConf))
	if err != nil {
		return nil, err
	}

	return total.ToCoin(), nil
}

// getMasterPubkey handles a getmasterpubkey request by returning the wallet
// master pubkey encoded as a string.
func getMasterPubkey(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetMasterPubkeyCmd)

	// If no account is passed, we provide the extended public key
	// for the default account number.
	account := uint32(udb.DefaultAccountNum)
	if cmd.Account != nil {
		var err error
		account, err = w.AccountNumber(*cmd.Account)
		if err != nil {
			return nil, err
		}
	}

	return w.MasterPubKey(account)
}

// getStakeInfo gets a large amounts of information about the stake environment
// and a number of statistics about local staking in the wallet.
func getStakeInfo(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	// Asynchronously query for the stake difficulty.
	sdiffFuture := chainClient.GetStakeDifficultyAsync()

	stakeInfo, err := w.StakeInfo(chainClient)
	if err != nil {
		return nil, err
	}

	proportionLive := float64(0.0)
	if float64(stakeInfo.PoolSize) > 0.0 {
		proportionLive = float64(stakeInfo.Live) / float64(stakeInfo.PoolSize)
	}
	proportionMissed := float64(0.0)
	if stakeInfo.Missed > 0 {
		proportionMissed = float64(stakeInfo.Missed) /
			(float64(stakeInfo.Voted) + float64(stakeInfo.Missed))
	}

	sdiff, err := sdiffFuture.Receive()
	if err != nil {
		return nil, err
	}

	resp := &hcjson.GetStakeInfoResult{
		BlockHeight:      stakeInfo.BlockHeight,
		PoolSize:         stakeInfo.PoolSize,
		Difficulty:       sdiff.NextStakeDifficulty,
		AllMempoolTix:    stakeInfo.AllMempoolTix,
		OwnMempoolTix:    stakeInfo.OwnMempoolTix,
		Immature:         stakeInfo.Immature,
		Live:             stakeInfo.Live,
		ProportionLive:   proportionLive,
		Voted:            stakeInfo.Voted,
		TotalSubsidy:     stakeInfo.TotalSubsidy.ToCoin(),
		Missed:           stakeInfo.Missed,
		ProportionMissed: proportionMissed,
		Revoked:          stakeInfo.Revoked,
		Expired:          stakeInfo.Expired,
	}

	return resp, nil
}

// getTicketFee gets the currently set price per kb for tickets
func getTicketFee(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return w.TicketFeeIncrement().ToCoin(), nil
}

// getTickets handles a gettickets request by returning the hashes of the tickets
// currently owned by wallet, encoded as strings.
func getTickets(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.GetTicketsCmd)

	ticketHashes, err := w.LiveTicketHashes(chainClient, cmd.IncludeImmature)
	if err != nil {
		return nil, err
	}

	// Compose a slice of strings to return.
	ticketHashStrs := make([]string, 0, len(ticketHashes))
	for i := range ticketHashes {
		ticketHashStrs = append(ticketHashStrs, ticketHashes[i].String())
	}

	return &hcjson.GetTicketsResult{Hashes: ticketHashStrs}, nil
}

// getTransaction handles a gettransaction request by returning details about
// a single transaction saved by wallet.
func getTransaction(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.GetTransactionCmd)

	txSha, err := chainhash.NewHashFromStr(cmd.Txid)
	if err != nil {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCDecodeHexString,
			Message: "Transaction hash string decode failed: " + err.Error(),
		}
	}

	txd, err := wallet.UnstableAPI(w).TxDetails(txSha)
	if err != nil {
		return nil, err
	}
	if txd == nil {
		return nil, &ErrNoTransactionInfo
	}

	_, tipHeight := w.MainChainTip()

	// TODO: The serialized transaction is already in the DB, so
	// reserializing can be avoided here.
	var txBuf bytes.Buffer
	txBuf.Grow(txd.MsgTx.SerializeSize())
	err = txd.MsgTx.Serialize(&txBuf)
	if err != nil {
		return nil, err
	}

	// TODO: Add a "generated" field to this result type.  "generated":true
	// is only added if the transaction is a coinbase.
	ret := hcjson.GetTransactionResult{
		TxID:            cmd.Txid,
		Hex:             hex.EncodeToString(txBuf.Bytes()),
		Time:            txd.Received.Unix(),
		TimeReceived:    txd.Received.Unix(),
		WalletConflicts: []string{}, // Not saved
		//Generated:     blockchain.IsCoinBaseTx(&txd.MsgTx),
	}

	if txd.Block.Height != -1 {
		ret.BlockHash = txd.Block.Hash.String()
		ret.BlockTime = txd.Block.Time.Unix()
		ret.Confirmations = int64(confirms(txd.Block.Height,
			tipHeight))
	}

	var (
		debitTotal  hcutil.Amount
		creditTotal hcutil.Amount // Excludes change
		fee         hcutil.Amount
		negFeeF64   float64
	)
	for _, deb := range txd.Debits {
		debitTotal += deb.Amount
	}
	for _, cred := range txd.Credits {
		if !cred.Change {
			creditTotal += cred.Amount
		}
	}
	// Fee can only be determined if every input is a debit.
	if len(txd.Debits) == len(txd.MsgTx.TxIn) {
		var outputTotal hcutil.Amount
		for _, output := range txd.MsgTx.TxOut {
			outputTotal += hcutil.Amount(output.Value)
		}
		fee = debitTotal - outputTotal
		negFeeF64 = (-fee).ToCoin()
	}
	ret.Amount = (creditTotal - debitTotal).ToCoin()
	ret.Fee = negFeeF64

	details, err := w.ListTransactionDetails(txSha)
	if err != nil {
		return nil, err
	}
	ret.Details = make([]hcjson.GetTransactionDetailsResult, len(details))
	for i, d := range details {
		ret.Details[i] = hcjson.GetTransactionDetailsResult{
			Account:           d.Account,
			Address:           d.Address,
			Amount:            d.Amount,
			Category:          d.Category,
			InvolvesWatchOnly: d.InvolvesWatchOnly,
			Fee:               d.Fee,
			Vout:              d.Vout,
		}
	}

	return ret, nil
}

// getVoteChoices handles a getvotechoices request by returning configured vote
// preferences for each agenda of the latest supported stake version.
func getVoteChoices(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	version, agendas := wallet.CurrentAgendas(w.ChainParams())
	resp := &hcjson.GetVoteChoicesResult{
		Version: version,
		Choices: make([]hcjson.VoteChoice, len(agendas)),
	}

	choices, _, err := w.AgendaChoices()
	if err != nil {
		return nil, err
	}

	for i := range choices {
		resp.Choices[i] = hcjson.VoteChoice{
			AgendaID:          choices[i].AgendaID,
			AgendaDescription: agendas[i].Vote.Description,
			ChoiceID:          choices[i].ChoiceID,
			ChoiceDescription: "", // Set below
		}
		for j := range agendas[i].Vote.Choices {
			if choices[i].ChoiceID == agendas[i].Vote.Choices[j].Id {
				resp.Choices[i].ChoiceDescription = agendas[i].Vote.Choices[j].Description
				break
			}
		}
	}

	return resp, nil
}

// getWalletFee returns the currently set tx fee for the requested wallet
func getWalletFee(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return w.RelayFee().ToCoin(), nil
}

// These generators create the following global variables in this package:
//
//   var localeHelpDescs map[string]func() map[string]string
//   var requestUsages string
//
// localeHelpDescs maps from locale strings (e.g. "en_US") to a function that
// builds a map of help texts for each RPC server method.  This prevents help
// text maps for every locale map from being rooted and created during init.
// Instead, the appropiate function is looked up when help text is first needed
// using the current locale and saved to the global below for futher reuse.
//
// requestUsages contains single line usages for every supported request,
// separated by newlines.  It is set during init.  These usages are used for all
// locales.
//
//go:generate go run ../../internal/rpchelp/genrpcserverhelp.go legacyrpc
//go:generate gofmt -w rpcserverhelp.go

var helpDescs map[string]string
var helpDescsMu sync.Mutex // Help may execute concurrently, so synchronize access.

// helpWithChainRPC handles the help request when the RPC server has been
// associated with a consensus RPC client.  The additional RPC client is used to
// include help messages for methods implemented by the consensus server via RPC
// passthrough.
func helpWithChainRPC(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	return help(icmd, w, chainClient)
}

// helpNoChainRPC handles the help request when the RPC server has not been
// associated with a consensus RPC client.  No help messages are included for
// passthrough requests.
func helpNoChainRPC(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return help(icmd, w, nil)
}

// help handles the help request by returning one line usage of all available
// methods, or full help for a specific method.  The chainClient is optional,
// and this is simply a helper function for the HelpNoChainRPC and
// HelpWithChainRPC handlers.
func help(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.HelpCmd)

	if cmd.Command == nil || *cmd.Command == "" {
		// Prepend chain server usage if it is available.
		usages := requestUsages
		if chainClient != nil {
			rawChainUsage, err := chainClient.RawRequest("help", nil)
			var chainUsage string
			if err == nil {
				_ = json.Unmarshal([]byte(rawChainUsage), &chainUsage)
			}
			if chainUsage != "" {
				usages = "Chain server usage:\n\n" + chainUsage + "\n\n" +
					"Wallet server usage (overrides chain requests):\n\n" +
					requestUsages
			}
		}
		return usages, nil
	}

	defer helpDescsMu.Unlock()
	helpDescsMu.Lock()

	if helpDescs == nil {
		// TODO: Allow other locales to be set via config or detemine
		// this from environment variables.  For now, hardcode US
		// English.
		helpDescs = localeHelpDescs["en_US"]()
	}

	helpText, ok := helpDescs[*cmd.Command]
	if ok {
		return helpText, nil
	}

	// Return the chain server's detailed help if possible.
	var chainHelp string
	if chainClient != nil {
		param := make([]byte, len(*cmd.Command)+2)
		param[0] = '"'
		copy(param[1:], *cmd.Command)
		param[len(param)-1] = '"'
		rawChainHelp, err := chainClient.RawRequest("help", []json.RawMessage{param})
		if err == nil {
			_ = json.Unmarshal([]byte(rawChainHelp), &chainHelp)
		}
	}
	if chainHelp != "" {
		return chainHelp, nil
	}
	return nil, &hcjson.RPCError{
		Code:    hcjson.ErrRPCInvalidParameter,
		Message: fmt.Sprintf("No help for method '%s'", *cmd.Command),
	}
}

// listAccounts handles a listaccounts request by returning a map of account
// names to their balances.
func listAccounts(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListAccountsCmd)

	accountBalances := map[string]float64{}
	results, err := w.CalculateAccountBalances(int32(*cmd.MinConf))
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		accountName, err := w.AccountName(result.Account)
		if err != nil {
			return nil, err
		}
		accountBalances[accountName] = result.Spendable.ToCoin()
	}
	// Return the map.  This will be marshaled into a JSON object.
	return accountBalances, nil
}

// listLockUnspent handles a listlockunspent request by returning an slice of
// all locked outpoints.
func listLockUnspent(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return w.LockedOutpoints(), nil
}

// listReceivedByAccount handles a listreceivedbyaccount request by returning
// a slice of objects, each one containing:
//  "account": the receiving account;
//  "amount": total amount received by the account;
//  "confirmations": number of confirmations of the most recent transaction.
// It takes two parameters:
//  "minconf": minimum number of confirmations to consider a transaction -
//             default: one;
//  "includeempty": whether or not to include addresses that have no transactions -
//                  default: false.
func listReceivedByAccount(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListReceivedByAccountCmd)

	results, err := w.TotalReceivedForAccounts(int32(*cmd.MinConf))
	if err != nil {
		return nil, err
	}

	jsonResults := make([]hcjson.ListReceivedByAccountResult, 0, len(results))
	for _, result := range results {
		jsonResults = append(jsonResults, hcjson.ListReceivedByAccountResult{
			Account:       result.AccountName,
			Amount:        result.TotalReceived.ToCoin(),
			Confirmations: uint64(result.LastConfirmation),
		})
	}
	return jsonResults, nil
}

// listReceivedByAddress handles a listreceivedbyaddress request by returning
// a slice of objects, each one containing:
//  "account": the account of the receiving address;
//  "address": the receiving address;
//  "amount": total amount received by the address;
//  "confirmations": number of confirmations of the most recent transaction.
// It takes two parameters:
//  "minconf": minimum number of confirmations to consider a transaction -
//             default: one;
//  "includeempty": whether or not to include addresses that have no transactions -
//                  default: false.
func listReceivedByAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListReceivedByAddressCmd)

	// Intermediate data for each address.
	type AddrData struct {
		// Total amount received.
		amount hcutil.Amount
		// Number of confirmations of the last transaction.
		confirmations int32
		// Hashes of transactions which include an output paying to the address
		tx []string
	}

	_, tipHeight := w.MainChainTip()

	// Intermediate data for all addresses.
	allAddrData := make(map[string]AddrData)
	// Create an AddrData entry for each active address in the account.
	// Otherwise we'll just get addresses from transactions later.
	sortedAddrs, err := w.SortedActivePaymentAddresses()
	if err != nil {
		return nil, err
	}
	for _, address := range sortedAddrs {
		// There might be duplicates, just overwrite them.
		allAddrData[address] = AddrData{}
	}

	minConf := *cmd.MinConf
	var endHeight int32
	if minConf == 0 {
		endHeight = -1
	} else {
		endHeight = tipHeight - int32(minConf) + 1
	}
	err = wallet.UnstableAPI(w).RangeTransactions(0, endHeight, func(details []udb.TxDetails) (bool, error) {
		confirmations := confirms(details[0].Block.Height, tipHeight)
		for _, tx := range details {
			for _, cred := range tx.Credits {
				pkVersion := tx.MsgTx.TxOut[cred.Index].Version
				pkScript := tx.MsgTx.TxOut[cred.Index].PkScript
				_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkVersion,
					pkScript, w.ChainParams())
				if err != nil {
					// Non standard script, skip.
					continue
				}
				for _, addr := range addrs {
					addrStr := addr.EncodeAddress()
					addrData, ok := allAddrData[addrStr]
					if ok {
						addrData.amount += cred.Amount
						// Always overwrite confirmations with newer ones.
						addrData.confirmations = confirmations
					} else {
						addrData = AddrData{
							amount:        cred.Amount,
							confirmations: confirmations,
						}
					}
					addrData.tx = append(addrData.tx, tx.Hash.String())
					allAddrData[addrStr] = addrData
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	// Massage address data into output format.
	numAddresses := len(allAddrData)
	ret := make([]hcjson.ListReceivedByAddressResult, numAddresses)
	idx := 0
	for address, addrData := range allAddrData {
		ret[idx] = hcjson.ListReceivedByAddressResult{
			Address:       address,
			Amount:        addrData.amount.ToCoin(),
			Confirmations: uint64(addrData.confirmations),
			TxIDs:         addrData.tx,
		}
		idx++
	}
	return ret, nil
}

// listSinceBlock handles a listsinceblock request by returning an array of maps
// with details of sent and received wallet transactions since the given block.
func listSinceBlock(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.ListSinceBlockCmd)

	_, tipHeight := w.MainChainTip()
	targetConf := int64(*cmd.TargetConfirmations)

	// For the result we need the block hash for the last block counted
	// in the blockchain due to confirmations. We send this off now so that
	// it can arrive asynchronously while we figure out the rest.
	gbh := chainClient.GetBlockHashAsync(int64(tipHeight) + 1 - targetConf)

	var start int32
	if cmd.BlockHash != nil {
		hash, err := chainhash.NewHashFromStr(*cmd.BlockHash)
		if err != nil {
			return nil, DeserializationError{err}
		}
		block, err := chainClient.GetBlockVerbose(hash, false)
		if err != nil {
			return nil, err
		}
		start = int32(block.Height) + 1
	}

	txInfoList, err := w.ListSinceBlock(start, -1, tipHeight)
	if err != nil {
		return nil, err
	}

	// Done with work, get the response.
	blockHash, err := gbh.Receive()
	if err != nil {
		return nil, err
	}

	res := hcjson.ListSinceBlockResult{
		Transactions: txInfoList,
		LastBlock:    blockHash.String(),
	}
	return res, nil
}

// listScripts handles a listscripts request by returning an
// array of script details for all scripts in the wallet.
func listScripts(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	redeemScripts, err := w.FetchAllRedeemScripts()
	if err != nil {
		return nil, err
	}
	listScriptsResultSIs := make([]hcjson.ScriptInfo, len(redeemScripts))
	for i, redeemScript := range redeemScripts {
		p2shAddr, err := hcutil.NewAddressScriptHash(redeemScript,
			w.ChainParams())
		if err != nil {
			return nil, err
		}
		listScriptsResultSIs[i] = hcjson.ScriptInfo{
			Hash160:      hex.EncodeToString(p2shAddr.Hash160()[:]),
			Address:      p2shAddr.EncodeAddress(),
			RedeemScript: hex.EncodeToString(redeemScript),
		}
	}
	return &hcjson.ListScriptsResult{Scripts: listScriptsResultSIs}, nil
}

// listTransactions handles a listtransactions request by returning an
// array of maps with details of sent and recevied wallet transactions.
func listTransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListTransactionsCmd)

	// TODO: ListTransactions does not currently understand the difference
	// between transactions pertaining to one account from another.  This
	// will be resolved when wtxmgr is combined with the waddrmgr namespace.

	if cmd.Account != nil && *cmd.Account != "*" {
		// For now, don't bother trying to continue if the user
		// specified an account, since this can't be (easily or
		// efficiently) calculated.
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCWallet,
			Message: "Transactions are not yet grouped by account",
		}
	}

	return w.ListTransactions(*cmd.From, *cmd.Count)
}

// listAddressTransactions handles a listaddresstransactions request by
// returning an array of maps with details of spent and received wallet
// transactions.  The form of the reply is identical to listtransactions,
// but the array elements are limited to transaction details which are
// about the addresess included in the request.
func listAddressTransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListAddressTransactionsCmd)

	if cmd.Account != nil && *cmd.Account != "*" {
		return nil, &hcjson.RPCError{
			Code: hcjson.ErrRPCInvalidParameter,
			Message: "Listing transactions for addresses may only " +
				"be done for all accounts",
		}
	}

	// Decode addresses.
	hash160Map := make(map[string]struct{})
	for _, addrStr := range cmd.Addresses {
		addr, err := decodeAddress(addrStr, w.ChainParams())
		if err != nil {
			return nil, err
		}
		hash160Map[string(addr.ScriptAddress())] = struct{}{}
	}

	return w.ListAddressTransactions(hash160Map)
}

// listAllTransactions handles a listalltransactions request by returning
// a map with details of sent and recevied wallet transactions.  This is
// similar to ListTransactions, except it takes only a single optional
// argument for the account name and replies with all transactions.
func listAllTransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListAllTransactionsCmd)

	if cmd.Account != nil && *cmd.Account != "*" {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCInvalidParameter,
			Message: "Listing all transactions may only be done for all accounts",
		}
	}

	return w.ListAllTransactions()
}

// listUnspent handles the listunspent command.
func listUnspent(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ListUnspentCmd)

	var addresses map[string]struct{}
	if cmd.Addresses != nil {
		addresses = make(map[string]struct{})
		// confirm that all of them are good:
		for _, as := range *cmd.Addresses {
			a, err := decodeAddress(as, w.ChainParams())
			if err != nil {
				return nil, err
			}
			addresses[a.EncodeAddress()] = struct{}{}
		}
	}

	return w.ListUnspent(int32(*cmd.MinConf), int32(*cmd.MaxConf), addresses)
}

// lockUnspent handles the lockunspent command.
func lockUnspent(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.LockUnspentCmd)

	switch {
	case cmd.Unlock && len(cmd.Transactions) == 0:
		w.ResetLockedOutpoints()
	default:
		for _, input := range cmd.Transactions {
			txSha, err := chainhash.NewHashFromStr(input.Txid)
			if err != nil {
				return nil, ParseError{err}
			}
			op := wire.OutPoint{Hash: *txSha, Index: input.Vout}
			if cmd.Unlock {
				w.UnlockOutpoint(op)
			} else {
				w.LockOutpoint(op)
			}
		}
	}
	return true, nil
}

// purchaseTicket indicates to the wallet that a ticket should be purchased
// using all currently available funds. If the ticket could not be purchased
// because there are not enough eligible funds, an error will be returned.
func purchaseTicket(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	// Enforce valid and positive spend limit.
	cmd := icmd.(*hcjson.PurchaseTicketCmd)
	spendLimit, err := hcutil.NewAmount(cmd.SpendLimit)
	if err != nil {
		return nil, err
	}
	if spendLimit < 0 {
		return nil, ErrNeedPositiveSpendLimit
	}

	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Override the minimum number of required confirmations if specified
	// and enforce it is positive.
	minConf := int32(1)
	if cmd.MinConf != nil {
		minConf = int32(*cmd.MinConf)
		if minConf < 0 {
			return nil, ErrNeedPositiveMinconf
		}
	}

	// Set ticket address if specified.
	var ticketAddr hcutil.Address
	if cmd.TicketAddress != nil {
		if *cmd.TicketAddress != "" {
			if bytes.Equal([]byte((*cmd.TicketAddress)[0:2]), []byte("Hb")) {
				return nil, fmt.Errorf("postquantum addresses not yet supported")
			}
			addr, err := decodeAddress(*cmd.TicketAddress, w.ChainParams())
			if err != nil {
				return nil, err
			}
			ticketAddr = addr
		}
	}

	numTickets := 1
	if cmd.NumTickets != nil {
		if *cmd.NumTickets > 1 {
			numTickets = *cmd.NumTickets
		}
	}

	// Set pool address if specified.
	var poolAddr hcutil.Address
	var poolFee float64
	if cmd.PoolAddress != nil {
		if *cmd.PoolAddress != "" {
			addr, err := decodeAddress(*cmd.PoolAddress, w.ChainParams())
			if err != nil {
				return nil, err
			}
			poolAddr = addr

			// Attempt to get the amount to send to
			// the pool after.
			if cmd.PoolFees == nil {
				return nil, fmt.Errorf("gave pool address but no pool fee")
			}
			poolFee = *cmd.PoolFees
			err = txrules.IsValidPoolFeeRate(poolFee)
			if err != nil {
				return nil, err
			}
		}
	}

	// Set the expiry if specified.
	expiry := int32(0)
	if cmd.Expiry != nil {
		expiry = int32(*cmd.Expiry)
	}

	ticketFee := w.TicketFeeIncrement()
	// Set the ticket fee if specified.
	if cmd.TicketFee != nil {
		ticketFee, err = hcutil.NewAmount(*cmd.TicketFee)
		if err != nil {
			return nil, err
		}
	}

	hashes, err := w.PurchaseTickets(0, spendLimit, minConf, ticketAddr,
		account, numTickets, poolAddr, poolFee, expiry, w.RelayFee(),
		ticketFee)
	if err != nil {
		return nil, err
	}

	hashStrs := make([]string, len(hashes))
	for i := range hashes {
		hashStrs[i] = hashes[i].String()
	}

	return hashStrs, err
}

// makeOutputs creates a slice of transaction outputs from a pair of address
// strings to amounts.  This is used to create the outputs to include in newly
// created transactions from a JSON object describing the output destinations
// and amounts.
func makeOutputs(pairs map[string]hcutil.Amount, chainParams *chaincfg.Params) ([]*wire.TxOut, error) {
	outputs := make([]*wire.TxOut, 0, len(pairs))
	for addrStr, amt := range pairs {
		addr, err := decodeAddress(addrStr, chainParams)
		if err != nil {
			return nil, err
		}

		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, fmt.Errorf("cannot create txout script: %s", err)
		}

		outputs = append(outputs, wire.NewTxOut(int64(amt), pkScript))
	}
	return outputs, nil
}

// sendPairs creates and sends payment transactions.
// It returns the transaction hash in string format upon success
// All errors are returned in hcjson.RPCError format
func sendPairs(w *wallet.Wallet, amounts map[string]hcutil.Amount,
	account uint32, minconf int32, changeAddr string, payLoad []byte, fromAddress string) (string, error) {
	outputs, err := makeOutputs(amounts, w.ChainParams())
	if err != nil {
		return "", err
	}

	payloadOutput, err := w.MakeNulldataOutput(payLoad)
	if err != nil {
		return "", err
	}
	outputs = append(outputs, payloadOutput)

	txSha, err := w.SendOutputs(outputs, account, minconf, changeAddr, fromAddress)
	if err != nil {
		if err == txrules.ErrAmountNegative {
			return "", ErrNeedPositiveAmount
		}
		if apperrors.IsError(err, apperrors.ErrLocked) {
			return "", &ErrWalletUnlockNeeded
		}
		switch err.(type) {
		case hcjson.RPCError:
			return "", err
		}

		return "", &hcjson.RPCError{
			Code:    hcjson.ErrRPCInternal.Code,
			Message: err.Error(),
		}
	}

	return txSha.String(), err
}

// redeemMultiSigOut receives a transaction hash/idx and fetches the first output
// index or indices with known script hashes from the transaction. It then
// construct a transaction with a single P2PKH paying to a specified address.
// It signs any inputs that it can, then provides the raw transaction to
// the user to export to others to sign.
func redeemMultiSigOut(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.RedeemMultiSigOutCmd)

	// Convert the address to a useable format. If
	// we have no address, create a new address in
	// this wallet to send the output to.
	var addr hcutil.Address
	var err error
	if cmd.Address != nil {
		addr, err = decodeAddress(*cmd.Address, w.ChainParams())
		if err != nil {
			return nil, err
		}
	} else {
		account := uint32(udb.DefaultAccountNum)
		addr, err = w.NewInternalAddress(account, wallet.WithGapPolicyWrap())
		if err != nil {
			return nil, err
		}
	}

	// Lookup the multisignature output and get the amount
	// along with the script for that transaction. Then,
	// begin crafting a MsgTx.
	hash, err := chainhash.NewHashFromStr(cmd.Hash)
	if err != nil {
		return nil, err
	}
	op := wire.OutPoint{
		Hash:  *hash,
		Index: cmd.Index,
		Tree:  cmd.Tree,
	}
	p2shOutput, err := w.FetchP2SHMultiSigOutput(&op)
	if err != nil {
		return nil, err
	}
	sc := txscript.GetScriptClass(txscript.DefaultScriptVersion,
		p2shOutput.RedeemScript)
	if sc != txscript.MultiSigTy {
		return nil, fmt.Errorf("invalid P2SH script: not multisig")
	}
	var msgTx wire.MsgTx
	msgTx.AddTxIn(wire.NewTxIn(&op, nil))

	// Calculate the fees required, and make sure we have enough.
	// Then produce the txout.
	account := uint32(udb.DefaultAccountNum)
	size := wallet.EstimateTxSize(1, 1, account)
	feeEst := wallet.FeeForSize(w.RelayFee(), size)
	if feeEst >= p2shOutput.OutputAmount {
		return nil, fmt.Errorf("multisig out amt is too small "+
			"(have %v, %v fee suggested)", p2shOutput.OutputAmount, feeEst)
	}
	toReceive := p2shOutput.OutputAmount - feeEst
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, fmt.Errorf("cannot create txout script: %s", err)
	}
	msgTx.AddTxOut(wire.NewTxOut(int64(toReceive), pkScript))

	// Start creating the SignRawTransactionCmd.
	outpointScript, err := txscript.PayToScriptHashScript(p2shOutput.P2SHAddress.Hash160()[:])
	if err != nil {
		return nil, err
	}
	outpointScriptStr := hex.EncodeToString(outpointScript)

	rti := hcjson.RawTxInput{
		Txid:         cmd.Hash,
		Vout:         cmd.Index,
		Tree:         cmd.Tree,
		ScriptPubKey: outpointScriptStr,
		RedeemScript: "",
	}
	rtis := []hcjson.RawTxInput{rti}

	var buf bytes.Buffer
	buf.Grow(msgTx.SerializeSize())
	if err = msgTx.Serialize(&buf); err != nil {
		return nil, err
	}
	txDataStr := hex.EncodeToString(buf.Bytes())
	sigHashAll := "ALL"

	srtc := &hcjson.SignRawTransactionCmd{
		RawTx:    txDataStr,
		Inputs:   &rtis,
		PrivKeys: &[]string{},
		Flags:    &sigHashAll,
	}

	// Sign it and give the results to the user.
	signedTxResult, err := signRawTransaction(srtc, w, chainClient)
	if signedTxResult == nil || err != nil {
		return nil, err
	}
	srtTyped := signedTxResult.(hcjson.SignRawTransactionResult)
	return hcjson.RedeemMultiSigOutResult(srtTyped), nil
}

// redeemMultisigOuts receives a script hash (in the form of a
// script hash address), looks up all the unspent outpoints associated
// with that address, then generates a list of partially signed
// transactions spending to either an address specified or internal
// addresses in this wallet.
func redeemMultiSigOuts(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.RedeemMultiSigOutsCmd)

	// Get all the multisignature outpoints that are unspent for this
	// address.
	addr, err := decodeAddress(cmd.FromScrAddress, w.ChainParams())
	if err != nil {
		return nil, err
	}
	p2shAddr, ok := addr.(*hcutil.AddressScriptHash)
	if !ok {
		return nil, errors.New("address is not P2SH")
	}
	msos, err := wallet.UnstableAPI(w).UnspentMultisigCreditsForAddress(p2shAddr)
	if err != nil {
		return nil, err
	}
	max := uint32(0xffffffff)
	if cmd.Number != nil {
		max = uint32(*cmd.Number)
	}

	itr := uint32(0)
	len := math.Min(float64(max), float64(len(msos)))
	rmsoResults := make([]hcjson.RedeemMultiSigOutResult, int(len))
	for i, mso := range msos {
		if itr >= max {
			break
		}

		rmsoRequest := &hcjson.RedeemMultiSigOutCmd{
			Hash:    mso.OutPoint.Hash.String(),
			Index:   mso.OutPoint.Index,
			Tree:    mso.OutPoint.Tree,
			Address: cmd.ToAddress,
		}
		redeemResult, err := redeemMultiSigOut(rmsoRequest, w, chainClient)
		if err != nil {
			return nil, err
		}
		redeemResultTyped := redeemResult.(hcjson.RedeemMultiSigOutResult)
		rmsoResults[i] = redeemResultTyped

		itr++
	}

	return hcjson.RedeemMultiSigOutsResult{Results: rmsoResults}, nil
}

// rescanWallet initiates a rescan of the block chain for wallet data, blocking
// until the rescan completes or exits with an error.
func rescanWallet(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.RescanWalletCmd)
	if *cmd.BeginHeight != 0 {
		return nil, fmt.Errorf("not support sync from height != 0")
	}
	err := <-w.RescanFromHeight(chainClient, int32(*cmd.BeginHeight),true)
	return nil, err
}

// revokeTickets initiates the wallet to issue revocations for any missing tickets that
// not yet been revoked.
func revokeTickets(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	err := w.RevokeTickets(chainClient)
	return nil, err
}

// stakePoolUserInfo returns the ticket information for a given user from the
// stake pool.
func stakePoolUserInfo(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.StakePoolUserInfoCmd)

	userAddr, err := hcutil.DecodeAddress(cmd.User)
	if err != nil {
		return nil, err
	}
	spui, err := w.StakePoolUserInfo(userAddr)
	if err != nil {
		return nil, err
	}

	resp := new(hcjson.StakePoolUserInfoResult)
	for _, ticket := range spui.Tickets {
		var ticketRes hcjson.PoolUserTicket

		status := ""
		switch ticket.Status {
		case udb.TSImmatureOrLive:
			status = "live"
		case udb.TSVoted:
			status = "voted"
		case udb.TSMissed:
			status = "missed"
			if ticket.HeightSpent-ticket.HeightTicket >= w.ChainParams().TicketExpiry {
				status = "expired"
			}
		}
		ticketRes.Status = status

		ticketRes.Ticket = ticket.Ticket.String()
		ticketRes.TicketHeight = ticket.HeightTicket
		ticketRes.SpentBy = ticket.SpentBy.String()
		ticketRes.SpentByHeight = ticket.HeightSpent

		resp.Tickets = append(resp.Tickets, ticketRes)
	}
	for _, invalid := range spui.InvalidTickets {
		invalidTicket := invalid.String()

		resp.InvalidTickets = append(resp.InvalidTickets, invalidTicket)
	}

	return resp, nil
}

// ticketsForAddress retrieves all ticket hashes that have the passed voting
// address. It will only return tickets that are in the mempool or blockchain,
// and should not return pruned tickets.
func ticketsForAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.TicketsForAddressCmd)

	addr, err := hcutil.DecodeAddress(cmd.Address)
	if err != nil {
		return nil, err
	}

	ticketHashes, err := w.TicketHashesForVotingAddress(addr)
	if err != nil {
		return nil, err
	}

	ticketHashStrs := make([]string, 0, len(ticketHashes))
	for _, hash := range ticketHashes {
		ticketHashStrs = append(ticketHashStrs, hash.String())
	}

	return hcjson.TicketsForAddressResult{Tickets: ticketHashStrs}, nil
}

func isNilOrEmpty(s *string) bool {
	return s == nil || *s == ""
}

// sendFrom handles a sendfrom RPC request by creating a new transaction
// spending unspent transaction outputs for a wallet to another payment
// address.  Leftover inputs not sent to the payment address or a fee for
// the miner are sent back to a new address in the wallet.  Upon success,
// the TxID for the created transaction is returned.
func sendFrom(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SendFromCmd)

	// Transaction comments are not yet supported.  Error instead of
	// pretending to save them.
	if !isNilOrEmpty(cmd.Comment) || !isNilOrEmpty(cmd.CommentTo) {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCUnimplemented,
			Message: "Transaction comments are not yet supported",
		}
	}

	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Check that signed integer parameters are positive.
	if cmd.Amount < 0 {
		return nil, ErrNeedPositiveAmount
	}
	minConf := int32(*cmd.MinConf)
	if minConf < 0 {
		return nil, ErrNeedPositiveMinconf
	}
	// Create map of address and amount pairs.
	amt, err := hcutil.NewAmount(cmd.Amount)
	if err != nil {
		return nil, err
	}
	pairs := map[string]hcutil.Amount{
		cmd.ToAddress: amt,
	}

	return sendPairs(w, pairs, account, minConf, "", []byte{}, "")
}

// sendMany handles a sendmany RPC request by creating a new transaction
// spending unspent transaction outputs for a wallet to any number of
// payment addresses.  Leftover inputs not sent to the payment address
// or a fee for the miner are sent back to a new address in the wallet.
// Upon success, the TxID for the created transaction is returned.
func sendMany(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SendManyCmd)

	// Transaction comments are not yet supported.  Error instead of
	// pretending to save them.
	if !isNilOrEmpty(cmd.Comment) {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCUnimplemented,
			Message: "Transaction comments are not yet supported",
		}
	}

	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Check that minconf is positive.
	minConf := int32(*cmd.MinConf)
	if minConf < 0 {
		return nil, ErrNeedPositiveMinconf
	}

	// Recreate address/amount pairs, using hcutil.Amount.
	pairs := make(map[string]hcutil.Amount, len(cmd.Amounts))
	for k, v := range cmd.Amounts {
		amt, err := hcutil.NewAmount(v)
		if err != nil {
			return nil, err
		}
		pairs[k] = amt
	}

	return sendPairs(w, pairs, account, minConf, "", []byte{}, "")
}

// sendManyV2 handles a sendManyV2 RPC request by creating a new transaction
// spending unspent transaction outputs for a wallet to any number of
// payment addresses.  Leftover inputs not sent to the payment address
// or a fee for the miner are sent back to a designated  address or first address of the default account
// in the wallet. Upon success, the TxID for the created transaction is returned.
func sendManyV2(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SendManyV2Cmd)
	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Check that minconf is positive.
	minConf := int32(*cmd.MinConf)
	if minConf < 0 {
		return nil, ErrNeedPositiveMinconf
	}

	// Recreate address/amount pairs, using hcutil.Amount.
	pairs := make(map[string]hcutil.Amount, len(cmd.Amounts))
	for k, v := range cmd.Amounts {
		if v <= 0 {
			return nil, fmt.Errorf("send to addr:%s %v coin", k, v)
		}
		amt, err := hcutil.NewAmount(v)
		if err != nil {
			return nil, err
		}
		pairs[k] = amt
	}

	var changeAddr string
	if cmd.ChangeAddr == nil {
		addr, err := w.FirstAddr(account)
		if err != nil {
			return nil, err
		}
		changeAddr = addr.String()
	} else {
		changeAddr = *cmd.ChangeAddr
	}

	return sendPairs(w, pairs, account, minConf, changeAddr, []byte{}, "")
}

// sendToAddress handles a sendtoaddress RPC request by creating a new
// transaction spending unspent transaction outputs for a wallet to another
// payment address.  Leftover inputs not sent to the payment address or a fee
// for the miner are sent back to a new address in the wallet.  Upon success,
// the TxID for the created transaction is returned.
func sendToAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SendToAddressCmd)

	// Transaction comments are not yet supported.  Error instead of
	// pretending to save them.
	if !isNilOrEmpty(cmd.Comment) || !isNilOrEmpty(cmd.CommentTo) {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCUnimplemented,
			Message: "Transaction comments are not yet supported",
		}
	}

	account := uint32(udb.DefaultAccountNum)
	amt, err := hcutil.NewAmount(cmd.Amount)
	if err != nil {
		return nil, err
	}

	// Check that signed integer parameters are positive.
	if amt < 0 {
		return nil, ErrNeedPositiveAmount
	}

	// Mock up map of address and amount pairs.
	pairs := map[string]hcutil.Amount{
		cmd.Address: amt,
	}

	// sendtoaddress always spends from the default account, this matches bitcoind
	return sendPairs(w, pairs, account, 1, "", []byte{}, "")
}

// getStraightPubKey handles a getStraightPubKey RPC request by getting a straight public key
func getStraightPubKey(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.GetStraightPubKeyCmd)

	a, err := decodeAddress(cmd.SrcAddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	result := &hcjson.GetStraightPubKeyResult{
		StraightPubKey: "",
	}

	switch addr := a.(type) {
	case *hcutil.AddressPubKeyHash:
		pubKey, err := w.PubKeyForAddress(addr)
		if err != nil {
			return nil, err
		}
		enumECType := pubKey.GetType()
		switch enumECType {
		case chainec.ECTypeSecp256k1:
			pubKeyAddr, err := hcutil.NewAddressSecpPubKey(pubKey.Serialize(), w.ChainParams())
			if err != nil {
				return nil, err
			}
			result.StraightPubKey = pubKeyAddr.String()
		case bliss.BSTypeBliss:
			pubKeyAddr, err := hcutil.NewAddressBlissPubKey(pubKey.Serialize(), w.ChainParams())
			if err != nil {
				return nil, err
			}
			result.StraightPubKey = pubKeyAddr.String()
		default:
			return nil, errors.New("only secp256k1 and bliss " +
				"pubkeys are currently supported")
		}
		return result, nil
	default:
		return nil, errors.New("unknow error.")
	}
}

// sendToMultiSig handles a sendtomultisig RPC request by creating a new
// transaction spending amount many funds to an output containing a multi-
// signature script hash. The function will fail if there isn't at least one
// public key in the public key list that corresponds to one that is owned
// locally.
// Upon successfully sending the transaction to the daemon, the script hash
// is stored in the transaction manager and the corresponding address
// specified to be watched by the daemon.
// The function returns a tx hash, P2SH address, and a multisig script if
// successful.
// TODO Use with non-default accounts as well
func sendToMultiSig(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SendToMultiSigCmd)

	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	amount, err := hcutil.NewAmount(cmd.Amount)
	if err != nil {
		return nil, err
	}
	nrequired := int8(*cmd.NRequired)
	minconf := int32(*cmd.MinConf)
	pubkeys := make([]hcutil.Address, len(cmd.Pubkeys))

	// The address list will made up either of addreseses (pubkey hash), for
	// which we need to look up the keys in wallet, straight pubkeys, or a
	// mixture of the two.
	for i, a := range cmd.Pubkeys {
		// Try to parse as pubkey address.
		a, err := decodeAddress(a, w.ChainParams())
		if err != nil {
			return nil, err
		}

		switch addr := a.(type) {
		case *hcutil.AddressSecpPubKey:
			pubkeys[i] = addr
		case *hcutil.AddressBlissPubKey:
			pubkeys[i] = addr
		default:
			pubKey, err := w.PubKeyForAddress(addr)
			if err != nil {
				return nil, err
			}
			enumECType := pubKey.GetType()
			switch enumECType {
			case chainec.ECTypeSecp256k1:
				pubKeyAddr, err := hcutil.NewAddressSecpPubKey(
					pubKey.Serialize(), w.ChainParams())
				if err != nil {
					return nil, err
				}
				pubkeys[i] = pubKeyAddr
			case bliss.BSTypeBliss:
				pubKeyAddr, err := hcutil.NewAddressBlissPubKey(
					pubKey.Serialize(), w.ChainParams())
				if err != nil {
					return nil, err
				}
				pubkeys[i] = pubKeyAddr
			default:
				return nil, errors.New("only secp256k1 and bliss " +
					"pubkeys are currently supported")
			}
		}
	}

	ctx, addr, script, err :=
		w.CreateMultisigTx(account, amount, pubkeys, nrequired, minconf)
	if err != nil {
		return nil, fmt.Errorf("CreateMultisigTx error: %v", err.Error())
	}

	result := &hcjson.SendToMultiSigResult{
		TxHash:       ctx.MsgTx.TxHash().String(),
		Address:      addr.EncodeAddress(),
		RedeemScript: hex.EncodeToString(script),
	}

	err = chainClient.LoadTxFilter(false, []hcutil.Address{addr}, nil)
	if err != nil {
		return nil, err
	}

	log.Infof("Successfully sent funds to multisignature output in "+
		"transaction %v", ctx.MsgTx.TxHash().String())

	return result, nil
}

// sendToSStx handles a sendtosstx RPC request by creating a new transaction
// payment addresses.  Leftover inputs not sent to the payment address
// or a fee for the miner are sent back to a new address in the wallet.
// Upon success, the TxID for the created transaction is returned.
// hcd TODO: Clean these up
func sendToSStx(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SendToSStxCmd)
	minconf := int32(*cmd.MinConf)

	account, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Check that minconf is positive.
	if minconf < 0 {
		return nil, ErrNeedPositiveMinconf
	}

	// Recreate address/amount pairs, using hcutil.Amount.
	pair := make(map[string]hcutil.Amount, len(cmd.Amounts))
	for k, v := range cmd.Amounts {
		pair[k] = hcutil.Amount(v)
	}
	// Get current block's height.
	_, tipHeight := w.MainChainTip()

	usedEligible := []udb.Credit{}
	eligible, err := w.FindEligibleOutputs(account, minconf, tipHeight)
	if err != nil {
		return nil, err
	}
	// check to properly find utxos from eligible to help signMsgTx later on
	for _, input := range cmd.Inputs {
		for _, allEligible := range eligible {

			if allEligible.Hash.String() == input.Txid &&
				allEligible.Index == input.Vout &&
				allEligible.Tree == input.Tree {
				usedEligible = append(usedEligible, allEligible)
				break
			}
		}
	}
	// Create transaction, replying with an error if the creation
	// was not successful.
	createdTx, err := w.CreateSStxTx(pair, usedEligible, cmd.Inputs,
		cmd.COuts, minconf)
	if err != nil {
		switch err {
		case wallet.ErrNonPositiveAmount:
			return nil, ErrNeedPositiveAmount
		default:
			return nil, err
		}
	}

	txSha, err := chainClient.SendRawTransaction(createdTx.MsgTx, w.AllowHighFees)
	if err != nil {
		return nil, err
	}
	log.Infof("Successfully sent SStx purchase transaction %v", txSha)
	return txSha.String(), nil
}

// sendToSSGen handles a sendtossgen RPC request by creating a new transaction
// spending a stake ticket and generating stake rewards.
// Upon success, the TxID for the created transaction is returned.
// hcd TODO: Clean these up
func sendToSSGen(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SendToSSGenCmd)

	_, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Get the tx hash for the ticket.
	ticketHash, err := chainhash.NewHashFromStr(cmd.TicketHash)
	if err != nil {
		return nil, err
	}

	// Get the block header hash that the SSGen tx votes on.
	blockHash, err := chainhash.NewHashFromStr(cmd.BlockHash)
	if err != nil {
		return nil, err
	}

	// Create transaction, replying with an error if the creation
	// was not successful.
	createdTx, err := w.CreateSSGenTx(*ticketHash, *blockHash,
		cmd.Height, cmd.VoteBits)
	if err != nil {
		switch err {
		case wallet.ErrNonPositiveAmount:
			return nil, ErrNeedPositiveAmount
		default:
			return nil, err
		}
	}

	txHash := createdTx.MsgTx.TxHash()

	log.Infof("Successfully sent transaction %v", txHash)
	return txHash.String(), nil
}

// sendToSSRtx handles a sendtossrtx RPC request by creating a new transaction
// spending a stake ticket and generating stake rewards.
// Upon success, the TxID for the created transaction is returned.
// hcd TODO: Clean these up
func sendToSSRtx(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SendToSSRtxCmd)

	_, err := w.AccountNumber(cmd.FromAccount)
	if err != nil {
		return nil, err
	}

	// Get the tx hash for the ticket.
	ticketHash, err := chainhash.NewHashFromStr(cmd.TicketHash)
	if err != nil {
		return nil, err
	}

	// Create transaction, replying with an error if the creation
	// was not successful.
	createdTx, err := w.CreateSSRtx(*ticketHash)
	if err != nil {
		switch err {
		case wallet.ErrNonPositiveAmount:
			return nil, ErrNeedPositiveAmount
		default:
			return nil, err
		}
	}

	txSha, err := chainClient.SendRawTransaction(createdTx.MsgTx, w.AllowHighFees)
	if err != nil {
		return nil, err
	}
	log.Infof("Successfully sent transaction %v", txSha)
	return txSha.String(), nil
}

// setTicketFee sets the transaction fee per kilobyte added to tickets.
func setTicketFee(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SetTicketFeeCmd)

	// Check that amount is not negative.
	if cmd.Fee < 0 {
		return nil, ErrNeedPositiveAmount
	}

	incr, err := hcutil.NewAmount(cmd.Fee)
	if err != nil {
		return nil, err
	}
	w.SetTicketFeeIncrement(incr)

	// A boolean true result is returned upon success.
	return true, nil
}

// setTxFee sets the transaction fee per kilobyte added to transactions.
func setTxFee(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SetTxFeeCmd)

	// Check that amount is not negative.
	if cmd.Amount < 0 {
		return nil, ErrNeedPositiveAmount
	}

	relayFee, err := hcutil.NewAmount(cmd.Amount)
	if err != nil {
		return nil, err
	}
	w.SetRelayFee(relayFee)

	// A boolean true result is returned upon success.
	return true, nil
}

// setVoteChoice handles a setvotechoice request by modifying the preferred
// choice for a voting agenda.
func setVoteChoice(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SetVoteChoiceCmd)
	_, err := w.SetAgendaChoices(wallet.AgendaChoice{
		AgendaID: cmd.AgendaID,
		ChoiceID: cmd.ChoiceID,
	})
	return nil, err
}

// signMessage signs the given message with the private key for the given
// address
func signMessage(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.SignMessageCmd)

	addr, err := decodeAddress(cmd.Address, w.ChainParams())
	if err != nil {
		return nil, err
	}
	sig, err := w.SignMessage(cmd.Message, addr)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func signRawTransactionNoChainRPC(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return signRawTransaction(icmd, w, nil)
}

// signRawTransaction handles the signrawtransaction command.
//
// chainClient may be nil, in which case it was called by the NoChainRPC
// variant.  It must be checked before all usage.
func signRawTransaction(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SignRawTransactionCmd)

	fmt.Printf("cmd:%#v", cmd)
	serializedTx, err := decodeHexStr(cmd.RawTx)
	if err != nil {
		return nil, err
	}
	tx := wire.NewMsgTx()
	err = tx.Deserialize(bytes.NewBuffer(serializedTx))
	if err != nil {
		e := errors.New("TX decode failed")
		return nil, DeserializationError{e}
	}

	var hashType txscript.SigHashType
	switch *cmd.Flags {
	case "ALL":
		hashType = txscript.SigHashAll
	case "NONE":
		hashType = txscript.SigHashNone
	case "SINGLE":
		hashType = txscript.SigHashSingle
	case "ALL|ANYONECANPAY":
		hashType = txscript.SigHashAll | txscript.SigHashAnyOneCanPay
	case "NONE|ANYONECANPAY":
		hashType = txscript.SigHashNone | txscript.SigHashAnyOneCanPay
	case "SINGLE|ANYONECANPAY":
		hashType = txscript.SigHashSingle | txscript.SigHashAnyOneCanPay
	case "ssgen": // Special case of SigHashAll
		hashType = txscript.SigHashAll
	case "ssrtx": // Special case of SigHashAll
		hashType = txscript.SigHashAll
	default:
		e := errors.New("Invalid sighash parameter")
		return nil, InvalidParameterError{e}
	}

	// TODO: really we probably should look these up with hcd anyway to
	// make sure that they match the blockchain if present.
	inputs := make(map[wire.OutPoint][]byte)
	scripts := make(map[string][]byte)
	var cmdInputs []hcjson.RawTxInput
	if cmd.Inputs != nil {
		cmdInputs = *cmd.Inputs
	}
	for _, rti := range cmdInputs {
		inputSha, err := chainhash.NewHashFromStr(rti.Txid)
		if err != nil {
			return nil, DeserializationError{err}
		}

		script, err := decodeHexStr(rti.ScriptPubKey)
		if err != nil {
			return nil, err
		}

		// redeemScript is only actually used iff the user provided
		// private keys. In which case, it is used to get the scripts
		// for signing. If the user did not provide keys then we always
		// get scripts from the wallet.
		// Empty strings are ok for this one and hex.DecodeString will
		// DTRT.
		// Note that redeemScript is NOT only the redeemscript
		// required to be appended to the end of a P2SH output
		// spend, but the entire signature script for spending
		// *any* outpoint with dummy values inserted into it
		// that can later be replacing by txscript's sign.
		if cmd.PrivKeys != nil && len(*cmd.PrivKeys) != 0 {
			redeemScript, err := decodeHexStr(rti.RedeemScript)
			if err != nil {
				return nil, err
			}

			addr, err := hcutil.NewAddressScriptHash(redeemScript,
				w.ChainParams())
			if err != nil {
				return nil, DeserializationError{err}
			}
			scripts[addr.String()] = redeemScript
		}
		inputs[wire.OutPoint{
			Hash:  *inputSha,
			Tree:  rti.Tree,
			Index: rti.Vout,
		}] = script
	}

	// Now we go and look for any inputs that we were not provided by
	// querying hcd with getrawtransaction. We queue up a bunch of async
	// requests and will wait for replies after we have checked the rest of
	// the arguments.
	requested := make(map[wire.OutPoint]hcrpcclient.FutureGetTxOutResult)
	for i, txIn := range tx.TxIn {
		// We don't need the first input of a stakebase tx, as it's garbage
		// anyway.
		if i == 0 && *cmd.Flags == "ssgen" {
			continue
		}

		// Did we get this outpoint from the arguments?
		if _, ok := inputs[txIn.PreviousOutPoint]; ok {
			continue
		}

		// Asynchronously request the output script.
		if chainClient == nil {
			return nil, &hcjson.RPCError{
				Code:    -1,
				Message: "Chain RPC is inactive",
			}
		}
		requested[txIn.PreviousOutPoint] = chainClient.GetTxOutAsync(
			&txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index, true)
	}

	// Parse list of private keys, if present. If there are any keys here
	// they are the keys that we may use for signing. If empty we will
	// use any keys known to us already.
	var keys map[string]*hcutil.WIF
	if cmd.PrivKeys != nil {
		keys = make(map[string]*hcutil.WIF)

		for _, key := range *cmd.PrivKeys {
			wif, err := hcutil.DecodeWIF(key)
			if err != nil {
				return nil, DeserializationError{err}
			}

			if !wif.IsForNet(w.ChainParams()) {
				s := "key network doesn't match wallet's"
				return nil, DeserializationError{errors.New(s)}
			}

			var addr hcutil.Address
			switch wif.DSA() {
			case chainec.ECTypeSecp256k1:
				addr, err = hcutil.NewAddressSecpPubKey(wif.SerializePubKey(),
					w.ChainParams())
				if err != nil {
					return nil, DeserializationError{err}
				}
			case chainec.ECTypeEdwards:
				addr, err = hcutil.NewAddressEdwardsPubKey(
					wif.SerializePubKey(),
					w.ChainParams())
				if err != nil {
					return nil, DeserializationError{err}
				}
			case chainec.ECTypeSecSchnorr:
				addr, err = hcutil.NewAddressSecSchnorrPubKey(
					wif.SerializePubKey(),
					w.ChainParams())
				if err != nil {
					return nil, DeserializationError{err}
				}
			}
			keys[addr.EncodeAddress()] = wif
		}
	}

	// We have checked the rest of the args. now we can collect the async
	// txs. TODO: If we don't mind the possibility of wasting work we could
	// move waiting to the following loop and be slightly more asynchronous.
	for outPoint, resp := range requested {
		result, err := resp.Receive()
		if err != nil {
			return nil, err
		}
		// gettxout returns JSON null if the output is found, but is spent by
		// another transaction in the main chain.
		if result == nil {
			continue
		}
		script, err := hex.DecodeString(result.ScriptPubKey.Hex)
		if err != nil {
			return nil, err
		}
		inputs[outPoint] = script
	}

	// All args collected. Now we can sign all the inputs that we can.
	// `complete' denotes that we successfully signed all outputs and that
	// all scripts will run to completion. This is returned as part of the
	// reply.
	signErrs, err := w.SignTransaction(tx, hashType, inputs, keys, scripts)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Grow(tx.SerializeSize())

	// All returned errors (not OOM, which panics) encounted during
	// bytes.Buffer writes are unexpected.
	if err = tx.Serialize(&buf); err != nil {
		panic(err)
	}

	signErrors := make([]hcjson.SignRawTransactionError, 0, len(signErrs))
	for _, e := range signErrs {
		input := tx.TxIn[e.InputIndex]
		signErrors = append(signErrors, hcjson.SignRawTransactionError{
			TxID:      input.PreviousOutPoint.Hash.String(),
			Vout:      input.PreviousOutPoint.Index,
			ScriptSig: hex.EncodeToString(input.SignatureScript),
			Sequence:  input.Sequence,
			Error:     e.Error.Error(),
		})
	}

	return hcjson.SignRawTransactionResult{
		Hex:      hex.EncodeToString(buf.Bytes()),
		Complete: len(signErrors) == 0,
		Errors:   signErrors,
	}, nil
}

// signRawTransactions handles the signrawtransactions command.
func signRawTransactions(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	cmd := icmd.(*hcjson.SignRawTransactionsCmd)

	// Sign each transaction sequentially and record the results.
	// Error out if we meet some unexpected failure.
	results := make([]hcjson.SignRawTransactionResult, len(cmd.RawTxs))
	for i, etx := range cmd.RawTxs {
		flagAll := "ALL"
		srtc := &hcjson.SignRawTransactionCmd{
			RawTx: etx,
			Flags: &flagAll,
		}
		result, err := signRawTransaction(srtc, w, chainClient)
		if err != nil {
			return nil, err
		}

		tResult := result.(hcjson.SignRawTransactionResult)
		results[i] = tResult
	}

	// If the user wants completed transactions to be automatically send,
	// do that now. Otherwise, construct the slice and return it.
	toReturn := make([]hcjson.SignedTransaction, len(cmd.RawTxs))

	if *cmd.Send {
		for i, result := range results {
			if result.Complete {
				// Slow/mem hungry because of the deserializing.
				serializedTx, err := decodeHexStr(result.Hex)
				if err != nil {
					return nil, err
				}
				msgTx := wire.NewMsgTx()
				err = msgTx.Deserialize(bytes.NewBuffer(serializedTx))
				if err != nil {
					e := errors.New("TX decode failed")
					return nil, DeserializationError{e}
				}
				sent := false
				hashStr := ""
				hash, err := chainClient.SendRawTransaction(msgTx, w.AllowHighFees)
				// If sendrawtransaction errors out (blockchain rule
				// issue, etc), continue onto the next transaction.
				if err == nil {
					sent = true
					hashStr = hash.String()
				}

				st := hcjson.SignedTransaction{
					SigningResult: result,
					Sent:          sent,
					TxHash:        &hashStr,
				}
				toReturn[i] = st
			} else {
				st := hcjson.SignedTransaction{
					SigningResult: result,
					Sent:          false,
					TxHash:        nil,
				}
				toReturn[i] = st
			}
		}
	} else { // Just return the results.
		for i, result := range results {
			st := hcjson.SignedTransaction{
				SigningResult: result,
				Sent:          false,
				TxHash:        nil,
			}
			toReturn[i] = st
		}
	}

	return &hcjson.SignRawTransactionsResult{Results: toReturn}, nil
}

// validateAddress handles the validateaddress command.
func validateAddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.ValidateAddressCmd)

	result := hcjson.ValidateAddressWalletResult{}
	addr, err := decodeAddress(cmd.Address, w.ChainParams())
	if err != nil {
		// Use result zero value (IsValid=false).
		return result, nil
	}

	// We could put whether or not the address is a script here,
	// by checking the type of "addr", however, the reference
	// implementation only puts that information if the script is
	// "ismine", and we follow that behaviour.
	result.Address = addr.EncodeAddress()
	result.IsValid = true

	ainfo, err := w.AddressInfo(addr)
	if err != nil {
		if apperrors.IsError(err, apperrors.ErrAddressNotFound) {
			// No additional information available about the address.
			return result, nil
		}
		return nil, err
	}

	// The address lookup was successful which means there is further
	// information about it available and it is "mine".
	result.IsMine = true
	acctName, err := w.AccountName(ainfo.Account())
	if err != nil {
		return nil, &ErrAccountNameNotFound
	}
	result.Account = acctName

	switch ma := ainfo.(type) {
	case udb.ManagedPubKeyAddress:
		var pubKeyBytes []byte
		var err error

		result.IsCompressed = ma.Compressed()
		result.PubKey = ma.ExportPubKey()
		pubKeyBytes, err = hex.DecodeString(result.PubKey)
		if err != nil {
			return nil, err
		}
		if len(pubKeyBytes) == 897 {
			pubKeyAddr, err := hcutil.NewAddressBlissPubKey(pubKeyBytes,
				w.ChainParams())
			if err != nil {
				return nil, err
			}
			result.PubKeyAddr = pubKeyAddr.String()
		} else {
			pubKeyAddr, err := hcutil.NewAddressSecpPubKey(pubKeyBytes,
				w.ChainParams())
			if err != nil {
				return nil, err
			}
			result.PubKeyAddr = pubKeyAddr.String()
		}

	case udb.ManagedScriptAddress:
		result.IsScript = true

		// The script is only available if the manager is unlocked, so
		// just break out now if there is an error.
		script, err := w.RedeemScriptCopy(addr)
		if err != nil {
			break
		}
		result.Hex = hex.EncodeToString(script)

		// This typically shouldn't fail unless an invalid script was
		// imported.  However, if it fails for any reason, there is no
		// further information available, so just set the script type
		// a non-standard and break out now.
		class, addrs, reqSigs, err := txscript.ExtractPkScriptAddrs(
			txscript.DefaultScriptVersion, script, w.ChainParams())
		if err != nil {
			result.Script = txscript.NonStandardTy.String()
			break
		}

		addrStrings := make([]string, len(addrs))
		for i, a := range addrs {
			addrStrings[i] = a.EncodeAddress()
		}
		result.Addresses = addrStrings

		// Multi-signature scripts also provide the number of required
		// signatures.
		result.Script = class.String()
		if class == txscript.MultiSigTy {
			result.SigsRequired = int32(reqSigs)
		}
	}

	return result, nil
}

// verifyMessage handles the verifymessage command by verifying the provided
// compact signature for the given address and message.
func verifyMessage(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.VerifyMessageCmd)

	var valid bool

	// Decode address and base64 signature from the request.
	addr, err := hcutil.DecodeAddress(cmd.Address)
	if err != nil {
		return nil, err
	}
	sig, err := base64.StdEncoding.DecodeString(cmd.Signature)
	if err != nil {
		return nil, err
	}

	// Addresses must have an associated secp256k1 private key and therefore
	// must be P2PK or P2PKH (P2SH is not allowed).
	switch a := addr.(type) {
	case *hcutil.AddressSecpPubKey:
	case *hcutil.AddressPubKeyHash:
		if a.DSA(a.Net()) != chainec.ECTypeSecp256k1 {
			goto WrongAddrKind
		}
	default:
		goto WrongAddrKind
	}

	valid, err = wallet.VerifyMessage(cmd.Message, addr, sig)
	if err != nil {
		// Mirror Bitcoin Core behavior, which treats all erorrs as an invalid
		// signature.
		return false, nil
	}
	return valid, nil

WrongAddrKind:
	return nil, InvalidParameterError{errors.New("address must be secp256k1 P2PK or P2PKH")}
}

// versionWithChainRPC handles the version request when the RPC server has been
// associated with a consensus RPC client.  The additional RPC client is used to
// include the version results of the consensus RPC server via RPC passthrough.
func versionWithChainRPC(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	return version(icmd, w, chainClient)
}

// versionNoChainRPC handles the version request when the RPC server has not
// been associated with a consesnus RPC client.  No version results are included
// for passphrough requests.
func versionNoChainRPC(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return version(icmd, w, nil)
}

// version handles the version command by returning the RPC API versions of the
// wallet and, optionally, the consensus RPC server as well if it is associated
// with the server.  The chainClient is optional, and this is simply a helper
// function for the versionWithChainRPC and versionNoChainRPC handlers.
func version(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	var resp map[string]hcjson.VersionResult
	if chainClient != nil {
		var err error
		resp, err = chainClient.Version()
		if err != nil {
			return nil, err
		}
	} else {
		resp = make(map[string]hcjson.VersionResult)
	}

	resp["hcwalletjsonrpcapi"] = hcjson.VersionResult{
		VersionString: jsonrpcSemverString,
		Major:         jsonrpcSemverMajor,
		Minor:         jsonrpcSemverMinor,
		Patch:         jsonrpcSemverPatch,
	}
	return resp, nil
}

// walletInfo gets the current information about the wallet. If the daemon
// is connected and fails to ping, the function will still return that the
// daemon is disconnected.
func walletInfo(icmd interface{}, w *wallet.Wallet, chainClient *hcrpcclient.Client) (interface{}, error) {
	connected := !(chainClient.Disconnected())
	if connected {
		err := chainClient.Ping()
		if err != nil {
			log.Warnf("Ping failed on connected daemon client: %s", err.Error())
			connected = false
		}
	}

	unlocked := !(w.Locked())
	fi := w.RelayFee()
	tfi := w.TicketFeeIncrement()
	tp := w.TicketPurchasingEnabled()
	voteBits := w.VoteBits()
	var voteVersion uint32
	_ = binary.Read(bytes.NewBuffer(voteBits.ExtendedBits[0:4]), binary.LittleEndian, &voteVersion)
	voting := w.VotingEnabled()

	return &hcjson.WalletInfoResult{
		DaemonConnected:  connected,
		Unlocked:         unlocked,
		TxFee:            fi.ToCoin(),
		TicketFee:        tfi.ToCoin(),
		TicketPurchasing: tp,
		VoteBits:         voteBits.Bits,
		VoteBitsExtended: hex.EncodeToString(voteBits.ExtendedBits),
		VoteVersion:      voteVersion,
		Voting:           voting,
	}, nil
}

// walletIsLocked handles the walletislocked extension request by
// returning the current lock state (false for unlocked, true for locked)
// of an account.
func walletIsLocked(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return w.Locked(), nil
}

// walletLock handles a walletlock request by locking the all account
// wallets, returning an error if any wallet is not encrypted (for example,
// a watching-only wallet).
func walletLock(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	w.Lock()
	return nil, nil
}

// walletPassphrase responds to the walletpassphrase request by unlocking
// the wallet.  The decryption key is saved in the wallet until timeout
// seconds expires, after which the wallet is locked.
func walletPassphrase(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.WalletPassphraseCmd)

	timeout := time.Second * time.Duration(cmd.Timeout)
	var unlockAfter <-chan time.Time
	if timeout != 0 {
		unlockAfter = time.After(timeout)
	}
	err := w.Unlock([]byte(cmd.Passphrase), unlockAfter)
	return nil, err
}

// walletPassphraseChange responds to the walletpassphrasechange request
// by unlocking all accounts with the provided old passphrase, and
// re-encrypting each private key with an AES key derived from the new
// passphrase.
//
// If the old passphrase is correct and the passphrase is changed, all
// wallets will be immediately locked.
func walletPassphraseChange(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.WalletPassphraseChangeCmd)

	err := w.ChangePrivatePassphrase([]byte(cmd.OldPassphrase),
		[]byte(cmd.NewPassphrase))
	if apperrors.IsError(err, apperrors.ErrWrongPassphrase) {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCWalletPassphraseIncorrect,
			Message: "Incorrect passphrase",
		}
	}
	return nil, err
}

// decodeHexStr decodes the hex encoding of a string, possibly prepending a
// leading '0' character if there is an odd number of bytes in the hex string.
// This is to prevent an error for an invalid hex string when using an odd
// number of bytes when calling hex.Decode.
func decodeHexStr(hexStr string) ([]byte, error) {
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, &hcjson.RPCError{
			Code:    hcjson.ErrRPCDecodeHexString,
			Message: "Hex string decode failed: " + err.Error(),
		}
	}
	return decoded, nil
}