// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package legacyrpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HcashOrg/hcd/hcjson"
	"github.com/HcashOrg/hcd/hcutil"
	"github.com/HcashOrg/hcwallet/apperrors"
	"github.com/HcashOrg/hcwallet/omnilib"
	"github.com/HcashOrg/hcwallet/wallet"
	"github.com/HcashOrg/hcwallet/wallet/txrules"
	"github.com/HcashOrg/hcwallet/wallet/udb"
)

const (
	MininumAmount = 1000000
)

func getOminiMethod() map[string]LegacyRpcHandler {
	return map[string]LegacyRpcHandler{

		"omni_getinfo":                     {handler: omni_getinfo}, //by ycj 20180915
		"omni_createpayload_simplesend":    {handler: omni_createpayload_simplesend},
		"omni_createpayload_issuancefixed": {handler: omni_createpayload_issuancefixed},
		"omni_listproperties":              {handler: omni_listproperties},

		"omni_sendissuancefixed": {handler: omniSendIssuanceFixed},
		"omni_getbalance":        {handler: omniGetBalance},
		"omni_send":              {handler: omniSend},

		"omni_senddexsell":                       {handler: OmniSenddexsell},
		"omni_senddexaccept":                     {handler: OmniSenddexaccept},
		"omni_sendissuancecrowdsale":             {handler: OmniSendissuancecrowdsale},
		"omni_sendissuancemanaged":               {handler: OmniSendissuancemanaged},
		"omni_sendsto":                           {handler: OmniSendsto},
		"omni_sendgrant":                         {handler: OmniSendgrant},
		"omni_sendrevoke":                        {handler: OmniSendrevoke},
		"omni_sendclosecrowdsale":                {handler: OmniSendclosecrowdsale},
		"omni_sendtrade":                         {handler: OmniSendtrade},
		"omni_sendcanceltradesbyprice":           {handler: OmniSendcanceltradesbyprice},
		"omni_sendcanceltradesbypair":            {handler: OmniSendcanceltradesbypair},
		"omni_sendcancelalltrades":               {handler: OmniSendcancelalltrades},
		"omni_sendchangeissuer":                  {handler: OmniSendchangeissuer},
		"omni_sendall":                           {handler: OmniSendall},
		"omni_sendenablefreezing":                {handler: OmniSendenablefreezing},
		"omni_senddisablefreezing":               {handler: OmniSenddisablefreezing},
		"omni_sendfreeze":                        {handler: OmniSendfreeze},
		"omni_sendunfreeze":                      {handler: OmniSendunfreeze},
		"omni_sendrawtx":                         {handler: OmniSendrawtx},
		"omni_funded_send":                       {handler: OmniFundedSend},
		"omni_funded_sendall":                    {handler: OmniFundedSendall},
		"omni_getallbalancesforid":               {handler: OmniGetallbalancesforid},
		"omni_getallbalancesforaddress":          {handler: OmniGetallbalancesforaddress},
		"omni_getwalletbalances":                 {handler: OmniGetwalletbalances},
		"omni_getwalletaddressbalances":          {handler: OmniGetwalletaddressbalances},
		"omni_gettransaction":                    {handler: OmniGettransaction},
		"omni_listtransactions":                  {handler: OmniListtransactions},
		"omni_listblocktransactions":             {handler: OmniListblocktransactions},
		"omni_listpendingtransactions":           {handler: OmniListpendingtransactions},
		"omni_getactivedexsells":                 {handler: OmniGetactivedexsells},
		"omni_getproperty":                       {handler: OmniGetproperty},
		"omni_getactivecrowdsales":               {handler: OmniGetactivecrowdsales},
		"omni_getcrowdsale":                      {handler: OmniGetcrowdsale},
		"omni_getgrants":                         {handler: OmniGetgrants},
		"omni_getsto":                            {handler: OmniGetsto},
		"omni_gettrade":                          {handler: OmniGettrade},
		"omni_getorderbook":                      {handler: OmniGetorderbook},
		"omni_gettradehistoryforpair":            {handler: OmniGettradehistoryforpair},
		"omni_gettradehistoryforaddress":         {handler: OmniGettradehistoryforaddress},
		"omni_getactivations":                    {handler: OmniGetactivations},
		"omni_getpayload":                        {handler: OmniGetpayload},
		"omni_getseedblocks":                     {handler: OmniGetseedblocks},
		"omni_getcurrentconsensushash":           {handler: OmniGetcurrentconsensushash},
		"omni_decodetransaction":                 {handler: OmniDecodetransaction},
		"omni_createrawtx_opreturn":              {handler: OmniCreaterawtxOpreturn},
		"omni_createrawtx_multisig":              {handler: OmniCreaterawtxMultisig},
		"omni_createrawtx_input":                 {handler: OmniCreaterawtxInput},
		"omni_createrawtx_reference":             {handler: OmniCreaterawtxReference},
		"omni_createrawtx_change":                {handler: OmniCreaterawtxChange},
		"omni_createpayload_sendall":             {handler: OmniCreatepayloadSendall},
		"omni_createpayload_dexsell":             {handler: OmniCreatepayloadDexsell},
		"omni_createpayload_dexaccept":           {handler: OmniCreatepayloadDexaccept},
		"omni_createpayload_sto":                 {handler: OmniCreatepayloadSto},
		"omni_createpayload_issuancecrowdsale":   {handler: OmniCreatepayloadIssuancecrowdsale},
		"omni_createpayload_issuancemanaged":     {handler: OmniCreatepayloadIssuancemanaged},
		"omni_createpayload_closecrowdsale":      {handler: OmniCreatepayloadClosecrowdsale},
		"omni_createpayload_grant":               {handler: OmniCreatepayloadGrant},
		"omni_createpayload_revoke":              {handler: OmniCreatepayloadRevoke},
		"omni_createpayload_changeissuer":        {handler: OmniCreatepayloadChangeissuer},
		"omni_createpayload_trade":               {handler: OmniCreatepayloadTrade},
		"omni_createpayload_canceltradesbyprice": {handler: OmniCreatepayloadCanceltradesbyprice},
		"omni_createpayload_canceltradesbypair":  {handler: OmniCreatepayloadCanceltradesbypair},
		"omni_createpayload_cancelalltrades":     {handler: OmniCreatepayloadCancelalltrades},
		"omni_createpayload_enablefreezing":      {handler: OmniCreatepayloadEnablefreezing},
		"omni_createpayload_disablefreezing":     {handler: OmniCreatepayloadDisablefreezing},
		"omni_createpayload_freeze":              {handler: OmniCreatepayloadFreeze},
		"omni_createpayload_unfreeze":            {handler: OmniCreatepayloadUnfreeze},
		"omni_getfeecache":                       {handler: OmniGetfeecache},
		"omni_getfeetrigger":                     {handler: OmniGetfeetrigger},
		"omni_getfeeshare":                       {handler: OmniGetfeeshare},
		"omni_getfeedistribution":                {handler: OmniGetfeedistribution},
		"omni_getfeedistributions":               {handler: OmniGetfeedistributions},
		"omni_setautocommit":                     {handler: OmniSetautocommit},
		"omni_rollback":                          {handler: OmniRollBack},
	}
}

func OmniRollBack(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.OmniRollBackCmd)
	err := w.RollBackOminiTransaction(cmd.Height, nil)

	return "", err
}

//add by ycj 20180915
//commonly used cmd request
func omni_cmdReq(icmd interface{}, w *wallet.Wallet) (json.RawMessage, error) {
	byteCmd, err := hcjson.MarshalCmd(1, icmd)
	if err != nil {
		return nil, err
	}

	strRsp := omnilib.JsonCmdReqHcToOm(string(byteCmd))

	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf(response.Error.Message)
	}
	return response.Result, nil
}

//
func omni_getinfo(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return omni_cmdReq(icmd, w)
}

func omni_createpayload_simplesend(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	cmd := icmd.(*hcjson.OmniCreatepayloadSimplesendCmd)
	byteCmd, err := hcjson.MarshalCmd(1, cmd)
	if err != nil {
		return nil, err
	}
	strReq := string(byteCmd)
	strRsp := omnilib.JsonCmdReqHcToOm(strReq)

	var response hcjson.Response
	_ = json.Unmarshal([]byte(strRsp), &response)

	return response.Result, nil
}

func omni_createpayload_issuancefixed(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return omni_cmdReq(icmd, w)
}

func omni_listproperties(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return omni_cmdReq(icmd, w)
}

func omniSendIssuanceFixed(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	txIdBytes, err := omni_cmdReq(icmd, w)
	sendIssueCmd := icmd.(*hcjson.OmniSendissuancefixedCmd)
	if err != nil {
		return nil, err
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   sendIssueCmd.Fromaddress,
		ToAddress:     sendIssueCmd.Fromaddress,
		ChangeAddress: sendIssueCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(sendParams, w, payLoad)
}

//
func sendIssuanceFixed(w *wallet.Wallet, payLoad []byte) (string, error) {
	account := uint32(udb.DefaultAccountNum)

	var changeAddr string
	addr, err := w.FirstAddr(account)
	if err != nil {
		return "", err
	}
	changeAddr = addr.String()
	dstAddr := changeAddr

	amt, err := hcutil.NewAmount(20)
	if err != nil {
		return "", err
	}
	// Mock up map of address and amount pairs.
	pairs := map[string]hcutil.Amount{
		dstAddr: hcutil.Amount(amt),
	}

	// sendtoaddress always spends from the default account, this matches bitcoind
	return sendPairsWithPayLoad(w, pairs, account, 1, changeAddr, payLoad, "")
}

// OmniSendchangeissuer Change the issuer on record of the given tokens.
// $ omnicore-cli "omni_sendchangeissuer" \     "1ARjWDkZ7kT9fwjPrjcQyvbXDkEySzKHwu" "3HTHRxu3aSDV4deakjC7VmsiUp7c6dfbvs" 3
func OmniSendchangeissuer(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniSendchangeissuerCmd := icmd.(*hcjson.OmniSendchangeissuerCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)

	pairs := map[string]hcutil.Amount{
		omniSendchangeissuerCmd.Toaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, omniSendchangeissuerCmd.Fromaddress, payLoad, omniSendchangeissuerCmd.Fromaddress)
}

// OmniSendenablefreezing Enables address freezing for a centrally managed property.
// $ omnicore-cli "omni_sendenablefreezing" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" 2
func OmniSendenablefreezing(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniSendenablefreezingCmd := icmd.(*hcjson.OmniSendenablefreezingCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniSendenablefreezingCmd.Fromaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, omniSendenablefreezingCmd.Fromaddress, payLoad, omniSendenablefreezingCmd.Fromaddress)
}

// OmniSenddisablefreezing Disables address freezing for a centrally managed property.,IMPORTANT NOTE:  Disabling freezing for a property will UNFREEZE all frozen addresses for that property!
// $ omnicore-cli "omni_senddisablefreezing" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" 2
func OmniSenddisablefreezing(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniSenddisablefreezingCmd := icmd.(*hcjson.OmniSenddisablefreezingCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniSenddisablefreezingCmd.Fromaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, omniSenddisablefreezingCmd.Fromaddress, payLoad, omniSenddisablefreezingCmd.Fromaddress)
}

// OmniSendfreeze Freeze an address for a centrally managed token.,Note: Only the issuer may freeze tokens, and only if the token is of the managed type with the freezing option enabled.
// $ omnicore-cli "omni_sendfreeze" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" "3HTHRxu3aSDV4deakjC7VmsiUp7c6dfbvs" 2 1000
func OmniSendfreeze(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniSendfreezeCmd := icmd.(*hcjson.OmniSendfreezeCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniSendfreezeCmd.Toaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, "", payLoad, omniSendfreezeCmd.Fromaddress)
}

// OmniSendunfreeze Unfreeze an address for a centrally managed token.,Note: Only the issuer may unfreeze tokens
// $ omnicore-cli "omni_sendunfreeze" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" "3HTHRxu3aSDV4deakjC7VmsiUp7c6dfbvs" 2 1000
func OmniSendunfreeze(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniSendunfreezeCmd := icmd.(*hcjson.OmniSendunfreezeCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniSendunfreezeCmd.Toaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, "", payLoad, omniSendunfreezeCmd.Fromaddress)
}

// OmniFundedSend Creates and sends a funded simple send transaction.,All bitcoins from the sender are consumed and if there are bitcoins missing, they are taken from the specified fee source. Change is sent to the fee source!
// $ omnicore-cli "omni_funded_send" "1DFa5bT6KMEr6ta29QJouainsjaNBsJQhH" \     "15cWrfuvMxyxGst2FisrQcvcpF48x6sXoH" 1 "100.0" \     "15Jhzz4omEXEyFKbdcccJwuVPea5LqsKM1"
func OmniFundedSend(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniFundedSendCmd := icmd.(*hcjson.OmniFundedSendCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniFundedSendCmd.Toaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, omniFundedSendCmd.Feeaddress, payLoad, omniFundedSendCmd.Fromaddress)
}

// OmniFundedSendall Creates and sends a transaction that transfers all available tokens in the given ecosystem to the recipient.,All bitcoins from the sender are consumed and if there are bitcoins missing, they are taken from the specified fee source. Change is sent to the fee source!
// $ omnicore-cli "omni_funded_sendall" "1DFa5bT6KMEr6ta29QJouainsjaNBsJQhH" \     "15cWrfuvMxyxGst2FisrQcvcpF48x6sXoH" 1 "15Jhzz4omEXEyFKbdcccJwuVPea5LqsKM1"
func OmniFundedSendall(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	omniFundedSendallCmd := icmd.(*hcjson.OmniFundedSendallCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	pairs := map[string]hcutil.Amount{
		omniFundedSendallCmd.Toaddress: MininumAmount,
	}
	return sendPairsWithPayLoad(w, pairs, account, 1, omniFundedSendallCmd.Feeaddress, payLoad, omniFundedSendallCmd.Fromaddress)
}

func omniGetBalance(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	return omni_cmdReq(icmd, w)
}

func omniSend(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniSendCmd := icmd.(*hcjson.OmniSendCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSendCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	_, err = decodeAddress(omniSendCmd.Toaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSendCmd.Fromaddress,
		ChangeAddress: omniSendCmd.Fromaddress,
		ToAddress:     omniSendCmd.Toaddress,
		Amount:        1,
	}
	final, err := omniSendToAddress(cmd, w, payLoad)
	if err != nil {
		return nil, err
	}
	//
	params := make([]interface{}, 0, 10)
	params = append(params, final)
	params = append(params, omniSendCmd.Fromaddress)
	params = append(params, 0)
	params = append(params, omniSendCmd.Propertyid)
	params = append(params, omniSendCmd.Amount)
	params = append(params, true)
	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	//construct omni variables
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
	return final, err
}

type SendFromAddressToAddress struct {
	FromAddress   string
	ToAddress     string
	ChangeAddress string
	Amount        float64
	Comment       *string
	CommentTo     *string
}

func omniSendToAddress(cmd *SendFromAddressToAddress, w *wallet.Wallet, payLoad []byte) (string, error) {
	// Transaction comments are not yet supported.  Error instead of
	// pretending to save them.
	if !isNilOrEmpty(cmd.Comment) || !isNilOrEmpty(cmd.CommentTo) {
		return "", &hcjson.RPCError{
			Code:    hcjson.ErrRPCUnimplemented,
			Message: "Transaction comments are not yet supported",
		}
	}

	account := uint32(udb.DefaultAccountNum)

	// Mock up map of address and amount pairs.
	pairs := map[string]hcutil.Amount{
		cmd.ToAddress: MininumAmount,
	}

	return sendPairsWithPayLoad(w, pairs, account, 1, cmd.ChangeAddress, payLoad, cmd.FromAddress)
}

// OmniGetwalletbalances Returns a list of the total token balances of the whole wallet.
// $ omnicore-cli "omni_getwalletbalances"
func OmniGetwalletbalances(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	addresses, err := w.FetchAddressesByAccount(account)
	if err != nil {
		return nil, err
	}

	req := omnilib.Request{
		Method: "omni_getwalletbalances",
		Params: []interface{}{addresses},
	}
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	strRsp := omnilib.JsonCmdReqHcToOm(string(bytes))
	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf(response.Error.Message)
	}
	return response.Result, nil
}

// OmniGetwalletaddressbalances Returns a list of all token balances for every wallet address.
// $ omnicore-cli "omni_getwalletaddressbalances"
func OmniGetwalletaddressbalances(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	account := uint32(udb.DefaultAccountNum)
	addresses, err := w.FetchAddressesByAccount(account)
	if err != nil {
		return nil, err
	}

	req := omnilib.Request{
		Method: "omni_getwalletaddressbalances",
		Params: []interface{}{addresses},
	}
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	strRsp := omnilib.JsonCmdReqHcToOm(string(bytes))
	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf(response.Error.Message)
	}
	return response.Result, nil
}

// OmniListblocktransactions Lists all Omni transactions in a block.
// $ omnicore-cli "omni_listblocktransactions" 279007
func OmniListblocktransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniListblocktransactionsCmd := icmd.(*hcjson.OmniListblocktransactionsCmd)
	account := uint32(udb.DefaultAccountNum)
	addresses, err := w.FetchAddressesByAccount(account)
	if err != nil {
		return nil, err
	}

	req := omnilib.Request{
		Method: "omni_listblocktransactions",
		Params: []interface{}{omniListblocktransactionsCmd.Height, addresses},
	}
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	strRsp := omnilib.JsonCmdReqHcToOm(string(bytes))
	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf(response.Error.Message)
	}
	return response.Result, nil
}

// OmniListpendingtransactions Returns a list of unconfirmed Omni transactions, pending in the memory pool.,Note: the validity of pending transactions is uncertain, and the state of the memory pool may change at any moment. It is recommended to check transactions after confirmation, and pending transactions should be considered as invalid.
// $ omnicore-cli "omni_listpendingtransactions"
func OmniListpendingtransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniListpendingtransactionsCmd := icmd.(*hcjson.OmniListpendingtransactionsCmd)
	account := uint32(udb.DefaultAccountNum)
	var addresses []string
	if omniListpendingtransactionsCmd.Address != nil {
		addresses = append(addresses, *omniListpendingtransactionsCmd.Address)
	} else {
		addresses1, err := w.FetchAddressesByAccount(account)
		if err != nil {
			return nil, err
		}
		addresses = addresses1
	}

	req := omnilib.Request{
		Method: "omni_listpendingtransactions",
		Params: []interface{}{addresses},
	}
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	strRsp := omnilib.JsonCmdReqHcToOm(string(bytes))
	var response hcjson.Response
	err = json.Unmarshal([]byte(strRsp), &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf(response.Error.Message)
	}
	return response.Result, nil
}

// sendPairsWithPayLoad creates and sends payment transactions.
// It returns the transaction hash in string format upon success
// All errors are returned in hcjson.RPCError format
func sendPairsWithPayLoad(w *wallet.Wallet, amounts map[string]hcutil.Amount, account uint32, minconf int32, changeAddr string, payLoad []byte, fromAddress string) (string, error) {
	outputs, err := makeOutputs(amounts, w.ChainParams())
	if err != nil {
		return "", err
	}
	payloadNullDataOutput, err := w.MakeNulldataOutput(payLoad)
	if err != nil {
		return "", err
	}

	outputs = append(outputs, payloadNullDataOutput)

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

// OmniGetproperty Returns details for about the tokens or smart property to lookup.
// $ omnicore-cli "omni_getproperty" 3
func OmniGetproperty(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniGetpropertyCmd := icmd.(*hcjson.OmniGetpropertyCmd)
	_, int32Height := w.MainChainTip()
	var height int64
	height = int64(int32Height)
	omniGetpropertyCmd.CurrentHeight = &height
	return omni_cmdReq(omniGetpropertyCmd, w)
}

func OmniReadAllTxHash(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//var cmd hcjson.OmniReadAllTxHashCmd
	return omni_cmdReq(icmd, w)
}

// code below was auto generated

// OmniSend Create and broadcast a simple send transaction.
// $ omnicore-cli "omni_send" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" "37FaKponF7zqoMLUjEiko25pDiuVH5YLEa" 1 "100.0"
func OmniSend(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniSendCmd)
	return omni_cmdReq(icmd, w)
}

// OmniSenddexsell Place, update or cancel a sell offer on the traditional distributed OMNI/BTC exchange.
// $ omnicore-cli "omni_senddexsell" "37FaKponF7zqoMLUjEiko25pDiuVH5YLEa" 1 "1.5" "0.75" 25 "0.0005" 1
func OmniSenddexsell(icmd interface{}, w *wallet.Wallet) (interface{}, error) {

	omniSenddexsellCmd := icmd.(*hcjson.OmniSenddexsellCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSenddexsellCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSenddexsellCmd.Fromaddress,
		ChangeAddress: omniSenddexsellCmd.Fromaddress,
		ToAddress:     omniSenddexsellCmd.Fromaddress,
		Amount:       MininumAmount,
	}
	txid, err := omniSendToAddress(cmd, w, payLoad)
	if err != nil {
		return nil, err
	}

	params := make([]interface{}, 0, 10)
	params = append(params, txid)
	params = append(params, omniSenddexsellCmd.Fromaddress)
	params = append(params, 20) //MSC_TYPE_TRADE_OFFER = 20,
	params = append(params, omniSenddexsellCmd.Propertyidforsale)
	params = append(params, omniSenddexsellCmd.Amountforsale)
	params = append(params, false)
	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON)) //construct omni variables

	return txid, err

}

// OmniSenddexaccept Create and broadcast an accept offer for the specified token and amount.
// $ omnicore-cli "omni_senddexaccept" \     "35URq1NN3xL6GeRKUP6vzaQVcxoJiiJKd8" "37FaKponF7zqoMLUjEiko25pDiuVH5YLEa" 1 "15.0"
func OmniSenddexaccept(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSenddexacceptCmd)
	//return omni_cmdReq(icmd, w)

	omniSenddexacceptCmd := icmd.(*hcjson.OmniSenddexacceptCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSenddexacceptCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSenddexacceptCmd.Fromaddress,
		ChangeAddress: omniSenddexacceptCmd.Fromaddress,
		ToAddress:     omniSenddexacceptCmd.Toaddress,
		Amount:        MininumAmount,  // > Minacceptfee
	}
	txid, err := omniSendToAddress(cmd, w, payLoad)
	if err != nil {
		return nil, err
	}

	return txid, err

}

// OmniSendissuancecrowdsale Create new tokens as crowdsale.
// $ omnicore-cli "omni_sendissuancecrowdsale" \     "3JYd75REX3HXn1vAU83YuGfmiPXW7BpYXo" 2 1 0 "Companies" "Bitcoin Mining" \     "Quantum Miner" "" "" 2 "100" 1483228800 30 2
func OmniSendissuancecrowdsale(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendissuancecrowdsaleCmd)
	//return omni_cmdReq(icmd, w)

	txIdBytes, err := omni_cmdReq(icmd, w)
	omniSendissuancecrowdsaleCmd := icmd.(*hcjson.OmniSendissuancecrowdsaleCmd)
	if err != nil {
		return nil, err
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendissuancecrowdsaleCmd.Fromaddress,
		ToAddress:     omniSendissuancecrowdsaleCmd.Fromaddress,
		ChangeAddress: omniSendissuancecrowdsaleCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(sendParams, w, payLoad)

}

// OmniSendissuancefixed Create new tokens with fixed supply.
// $ omnicore-cli "omni_sendissuancefixed" \     "3Ck2kEGLJtZw9ENj2tameMCtS3HB7uRar3" 2 1 0 "Companies" "Bitcoin Mining" \     "Quantum Miner" "" "" "1000000"
func OmniSendissuancefixed(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniSendissuancefixedCmd)
	return omni_cmdReq(icmd, w)
}

// OmniSendissuancemanaged Create new tokens with manageable supply.
// $ omnicore-cli "omni_sendissuancemanaged" \     "3HsJvhr9qzgRe3ss97b1QHs38rmaLExLcH" 2 1 0 "Companies" "Bitcoin Mining" "Quantum Miner" "" ""
func OmniSendissuancemanaged(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendissuancemanagedCmd)
	//return omni_cmdReq(icmd, w)
	txIdBytes, err := omni_cmdReq(icmd, w)
	sendIssueCmd := icmd.(*hcjson.OmniSendissuancemanagedCmd)
	if err != nil {
		return err, nil
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return err, nil
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return err, nil
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   sendIssueCmd.Fromaddress,
		ToAddress:     sendIssueCmd.Fromaddress,
		ChangeAddress: sendIssueCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(sendParams, w, payLoad)

}

// OmniSendsto Create and broadcast a send-to-owners transaction.
// $ omnicore-cli "omni_sendsto" \     "32Z3tJccZuqQZ4PhJR2hxHC3tjgjA8cbqz" "37FaKponF7zqoMLUjEiko25pDiuVH5YLEa" 3 "5000"
func OmniSendsto(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniSendCmd := icmd.(*hcjson.OmniSendstoCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSendCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	//	_, err = decodeAddress(omniSendCmd.Toaddress, w.ChainParams())
	//	if err != nil {
	//		return nil, err
	//	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSendCmd.Fromaddress,
		ChangeAddress: omniSendCmd.Fromaddress,
		ToAddress:     omniSendCmd.Fromaddress,
		Amount:        1,
	}
	final, err := omniSendToAddress(cmd, w, payLoad)
	if err != nil {
		return nil, err
	}
	//
	params := make([]interface{}, 0, 10)
	params = append(params, final)
	params = append(params, omniSendCmd.Fromaddress)
	params = append(params, 3)
	params = append(params, omniSendCmd.Propertyid)
	params = append(params, omniSendCmd.Amount)
	params = append(params, true)

	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	//construct omni variables
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
	return final, err
}

// OmniSendgrant Issue or grant new units of managed tokens.
// $ omnicore-cli "omni_sendgrant" "3HsJvhr9qzgRe3ss97b1QHs38rmaLExLcH" "" 51 "7000"
func OmniSendgrant(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniSendGrantCmd := icmd.(*hcjson.OmniSendgrantCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSendGrantCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	_, err = decodeAddress(omniSendGrantCmd.Toaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSendGrantCmd.Fromaddress,
		ChangeAddress: omniSendGrantCmd.Fromaddress,
		ToAddress:     omniSendGrantCmd.Toaddress,
		Amount:        1,
	}
	return omniSendToAddress(cmd, w, payLoad)
	/*
		final, err := omniSendToAddress(cmd, w, payLoad)
		if err != nil{
			return nil, err
		}
		//
		params := make([]interface{}, 0, 10)
		params = append(params, omniSendCmd.Fromaddress)
		params = append(params, omniSendCmd.Propertyid)
		params = append(params, omniSendCmd.Amount)
		params = append(params, final)
		params = append(params, 3)

		newCmd, err := hcjson.NewCmd("omni_padding_add", params...)
		if err != nil {
			return nil, err
		}
		marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(marshalledJSON))
		//construct omni variables
		omnilib.JsonCmdReqHcToOm(string(marshalledJSON))
		return final, err
	*/
}

// OmniSendrevoke Revoke units of managed tokens.
// $ omnicore-cli "omni_sendrevoke" "3HsJvhr9qzgRe3ss97b1QHs38rmaLExLcH" "" 51 "100"
func OmniSendrevoke(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniSendrevokeCmd := icmd.(*hcjson.OmniSendrevokeCmd)
	ret, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSendrevokeCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSendrevokeCmd.Fromaddress,
		ChangeAddress: omniSendrevokeCmd.Fromaddress,
		ToAddress:     omniSendrevokeCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(cmd, w, payLoad)
}

// OmniSendclosecrowdsale Manually close a crowdsale.
// $ omnicore-cli "omni_sendclosecrowdsale" "3JYd75REX3HXn1vAU83YuGfmiPXW7BpYXo" 70
func OmniSendclosecrowdsale(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendclosecrowdsaleCmd)
	//return omni_cmdReq(icmd, w)
	txIdBytes, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}

	omniSendclosecrowdsaleCmd := icmd.(*hcjson.OmniSendclosecrowdsaleCmd)

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendclosecrowdsaleCmd.Fromaddress,
		ToAddress:     omniSendclosecrowdsaleCmd.Fromaddress,
		ChangeAddress: omniSendclosecrowdsaleCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(sendParams, w, payLoad)
}

// OmniSendtrade Place a trade offer on the distributed token exchange.
// $ omnicore-cli "omni_sendtrade" "3BydPiSLPP3DR5cf726hDQ89fpqWLxPKLR" 31 "250.0" 1 "10.0"
func OmniSendtrade(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	txIdBytes, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}

	omniSendtradeCmd := icmd.(*hcjson.OmniSendtradeCmd)

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendtradeCmd.Fromaddress,
		ToAddress:     omniSendtradeCmd.Fromaddress,
		ChangeAddress: omniSendtradeCmd.Fromaddress,
		Amount:        1,
	}
	return omniSendToAddress(sendParams, w, payLoad)

}

// OmniSendcanceltradesbyprice Cancel offers on the distributed token exchange with the specified price.
// $ omnicore-cli "omni_sendcanceltradesbyprice" "3BydPiSLPP3DR5cf726hDQ89fpqWLxPKLR" 31 "100.0" 1 "5.0"
func OmniSendcanceltradesbyprice(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendcanceltradesbypriceCmd)
	//return omni_cmdReq(icmd, w)
	omniSendcanceltradesbypriceCmd := icmd.(*hcjson.OmniSendcanceltradesbypriceCmd)

	txIdBytes, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendcanceltradesbypriceCmd.Fromaddress,
		ToAddress:     omniSendcanceltradesbypriceCmd.Fromaddress,
		ChangeAddress: omniSendcanceltradesbypriceCmd.Fromaddress,
		Amount:        1,
	}
	txid, err := omniSendToAddress(sendParams, w, payLoad)
	if err != nil {
		return nil, err
	}

	params := make([]interface{}, 0, 10)
	params = append(params, txid)
	params = append(params, omniSendcanceltradesbypriceCmd.Fromaddress)
	params = append(params, 26) //MSC_TYPE_METADEX_CANCEL_PRICE = 26,
	params = append(params, omniSendcanceltradesbypriceCmd.Propertyidforsale)
	params = append(params, omniSendcanceltradesbypriceCmd.Amountforsale)
	params = append(params, false)
	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON)) //construct omni variables

	return txid, nil
}

// OmniSendcanceltradesbypair Cancel all offers on the distributed token exchange with the given currency pair.
// $ omnicore-cli "omni_sendcanceltradesbypair" "3BydPiSLPP3DR5cf726hDQ89fpqWLxPKLR" 1 31
func OmniSendcanceltradesbypair(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendcanceltradesbypairCmd)
	//return omni_cmdReq(icmd, w)
	omniSendcanceltradesbypairCmd := icmd.(*hcjson.OmniSendcanceltradesbypairCmd)

	txIdBytes, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendcanceltradesbypairCmd.Fromaddress,
		ToAddress:     omniSendcanceltradesbypairCmd.Fromaddress,
		ChangeAddress: omniSendcanceltradesbypairCmd.Fromaddress,
		Amount:        1,
	}

	txid, err := omniSendToAddress(sendParams, w, payLoad)
	if err != nil {
		return nil, err
	}

	params := make([]interface{}, 0, 10)
	params = append(params, txid)
	params = append(params, omniSendcanceltradesbypairCmd.Fromaddress)
	params = append(params, 27) //MSC_TYPE_METADEX_CANCEL_PAIR = 27,
	params = append(params, omniSendcanceltradesbypairCmd.Propertyidforsale)
	params = append(params, "0")
	params = append(params, false)
	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON)) //construct omni variables

	return txid, nil

}

// OmniSendcancelalltrades Cancel all offers on the distributed token exchange.
// $ omnicore-cli "omni_sendcancelalltrades" "3BydPiSLPP3DR5cf726hDQ89fpqWLxPKLR" 1
func OmniSendcancelalltrades(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	//_ = icmd.(*hcjson.OmniSendcancelalltradesCmd)
	//return omni_cmdReq(icmd, w)
	omniSendcancelalltradesCmd := icmd.(*hcjson.OmniSendcancelalltradesCmd)

	txIdBytes, err := omni_cmdReq(icmd, w)
	if err != nil {
		return nil, err
	}

	txidStr := ""
	err = json.Unmarshal(txIdBytes, &txidStr)
	if err != nil {
		return nil, err
	}

	payLoad, err := hex.DecodeString(txidStr)
	if err != nil {
		return nil, err
	}

	sendParams := &SendFromAddressToAddress{
		FromAddress:   omniSendcancelalltradesCmd.Fromaddress,
		ToAddress:     omniSendcancelalltradesCmd.Fromaddress,
		ChangeAddress: omniSendcancelalltradesCmd.Fromaddress,
		Amount:        1,
	}
	txid, err := omniSendToAddress(sendParams, w, payLoad)
	if err != nil {
		return nil, err
	}

	params := make([]interface{}, 0, 10)
	params = append(params, txid)
	params = append(params, omniSendcancelalltradesCmd.Fromaddress)
	params = append(params, 28) //MSC_TYPE_METADEX_CANCEL_ECOSYSTEM = 28,
	params = append(params, omniSendcancelalltradesCmd.Ecosystem)
	params = append(params, "0")
	params = append(params, false)
	newCmd, err := hcjson.NewCmd("omni_pending_add", params...)
	if err != nil {
		return nil, err
	}
	marshalledJSON, err := hcjson.MarshalCmd(1, newCmd)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(marshalledJSON))
	omnilib.JsonCmdReqHcToOm(string(marshalledJSON)) //construct omni variables

	return txid, nil
}

// OmniSendall Transfers all available tokens in the given ecosystem to the recipient.
// $ omnicore-cli "omni_sendall" "3M9qvHKtgARhqcMtM5cRT9VaiDJ5PSfQGY" "37FaKponF7zqoMLUjEiko25pDiuVH5YLEa" 2
func OmniSendall(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	omniSendallCmd := icmd.(*hcjson.OmniSendallCmd)
	ret, err := omni_cmdReq(icmd, w)

	if err != nil {
		return nil, err
	}
	hexStr := strings.Trim(string(ret), "\"")
	payLoad, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	_, err = decodeAddress(omniSendallCmd.Fromaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	_, err = decodeAddress(omniSendallCmd.Toaddress, w.ChainParams())
	if err != nil {
		return nil, err
	}

	cmd := &SendFromAddressToAddress{
		FromAddress:   omniSendallCmd.Fromaddress,
		ChangeAddress: omniSendallCmd.Fromaddress,
		ToAddress:     omniSendallCmd.Toaddress,
		Amount:        1,
	}
	return omniSendToAddress(cmd, w, payLoad)
}

// OmniSendrawtx Broadcasts a raw Omni Layer transaction.
// $ omnicore-cli "omni_sendrawtx" \     "1MCHESTptvd2LnNp7wmr2sGTpRomteAkq8" "000000000000000100000000017d7840" \     "1EqTta1Rt8ixAA32DuC29oukbsSWU62qAV"
func OmniSendrawtx(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniSendrawtxCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetinfo Returns various state information of the client and protocol.
// $ omnicore-cli "omni_getinfo"
func OmniGetinfo(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetinfoCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetbalance Returns the token balance for a given address and property.
// $ omnicore-cli "omni_getbalance", "1EXoDusjGwvnjZUyKkxZ4UHEf77z6A5S4P" 1
func OmniGetbalance(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetbalanceCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetallbalancesforid Returns a list of token balances for a given currency or property identifier.
// $ omnicore-cli "omni_getallbalancesforid" 1
func OmniGetallbalancesforid(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetallbalancesforidCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetallbalancesforaddress Returns a list of all token balances for a given address.
// $ omnicore-cli "omni_getallbalancesforaddress" "1EXoDusjGwvnjZUyKkxZ4UHEf77z6A5S4P"
func OmniGetallbalancesforaddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetallbalancesforaddressCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGettransaction Get detailed information about an Omni transaction.
// $ omnicore-cli "omni_gettransaction" "1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d"
func OmniGettransaction(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGettransactionCmd)
	return omni_cmdReq(icmd, w)
}

// OmniListtransactions List wallet transactions, optionally filtered by an address and block boundaries.
// $ omnicore-cli "omni_listtransactions"
func OmniListtransactions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniListtransactionsCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetactivedexsells Returns currently active offers on the distributed exchange.
// $ omnicore-cli "omni_getactivedexsells"
func OmniGetactivedexsells(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetactivedexsellsCmd)
	return omni_cmdReq(icmd, w)
}

// OmniListproperties Lists all tokens or smart properties.
// $ omnicore-cli "omni_listproperties"
func OmniListproperties(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniListpropertiesCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetactivecrowdsales Lists currently active crowdsales.
// $ omnicore-cli "omni_getactivecrowdsales"
func OmniGetactivecrowdsales(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetactivecrowdsalesCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetcrowdsale Returns information about a crowdsale.
// $ omnicore-cli "omni_getcrowdsale" 3 true
func OmniGetcrowdsale(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetcrowdsaleCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetgrants Returns information about granted and revoked units of managed tokens.
// $ omnicore-cli "omni_getgrants" 31
func OmniGetgrants(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetgrantsCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetsto Get information and recipients of a send-to-owners transaction.
// $ omnicore-cli "omni_getsto" "1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d" "*"
func OmniGetsto(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetstoCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGettrade Get detailed information and trade matches for orders on the distributed token exchange.
// $ omnicore-cli "omni_gettrade" "1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d"
func OmniGettrade(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGettradeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetorderbook List active offers on the distributed token exchange.
// $ omnicore-cli "omni_getorderbook" 2
func OmniGetorderbook(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetorderbookCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGettradehistoryforpair Retrieves the history of trades on the distributed token exchange for the specified market.
// $ omnicore-cli "omni_gettradehistoryforpair" 1 12 500
func OmniGettradehistoryforpair(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGettradehistoryforpairCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGettradehistoryforaddress Retrieves the history of orders on the distributed exchange for the supplied address.
// $ omnicore-cli "omni_gettradehistoryforaddress" "1MCHESTptvd2LnNp7wmr2sGTpRomteAkq8"
func OmniGettradehistoryforaddress(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGettradehistoryforaddressCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetactivations Returns pending and completed feature activations.
// $ omnicore-cli "omni_getactivations"
func OmniGetactivations(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetactivationsCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetpayload Get the payload for an Omni transaction.
// $ omnicore-cli "omni_getactivations" "1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d"
func OmniGetpayload(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetpayloadCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetseedblocks Returns a list of blocks containing Omni transactions for use in seed block filtering.,WARNING: The Exodus crowdsale is not stored in LevelDB, thus this is currently only safe to use to generate seed blocks after block 255365.
// $ omnicore-cli "omni_getseedblocks" 290000 300000
func OmniGetseedblocks(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetseedblocksCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetcurrentconsensushash Returns the consensus hash covering the state of the current block.
// $ omnicore-cli "omni_getcurrentconsensushash"
func OmniGetcurrentconsensushash(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetcurrentconsensushashCmd)
	return omni_cmdReq(icmd, w)
}

// OmniDecodetransaction Decodes an Omni transaction.,If the inputs of the transaction are not in the chain, then they must be provided, because the transaction inputs are used to identify the sender of a transaction.,A block height can be provided, which is used to determine the parsing rules.
// $ omnicore-cli "omni_decodetransaction" "010000000163af14ce6d477e1c793507e32a5b7696288fa89705c0d02a3f66beb3c \     5b8afee0100000000ffffffff02ac020000000000004751210261ea979f6a06f9dafe00fb1263ea0aca959875a7073556a088cdf \     adcd494b3752102a3fd0a8a067e06941e066f78d930bfc47746f097fcd3f7ab27db8ddf37168b6b52ae22020000000000001976a \     914946cb2e08075bcbaf157e47bcb67eb2b2339d24288ac00000000" \     "[{\"txid\":\"eeafb8c5b3be663f2ad0c00597a88f2896765b2ae30735791c7e476dce14af63\",\"vout\":1, \     \"scriptPubKey\":\"76a9149084c0bd89289bc025d0264f7f23148fb683d56c88ac\",\"value\":0.0001123}]"
func OmniDecodetransaction(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniDecodetransactionCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreaterawtxOpreturn Adds a payload with class C (op-return) encoding to the transaction.,If no raw transaction is provided, a new transaction is created.,If the data encoding fails, then the transaction is not modified.
// $ omnicore-cli "omni_createrawtx_opreturn" "01000000000000000000" "00000000000000020000000006dac2c0"
func OmniCreaterawtxOpreturn(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreaterawtxOpreturnCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreaterawtxMultisig Adds a payload with class B (bare-multisig) encoding to the transaction.,If no raw transaction is provided, a new transaction is created.,If the data encoding fails, then the transaction is not modified.
// $ omnicore-cli "omni_createrawtx_multisig" \     "0100000001a7a9402ecd77f3c9f745793c9ec805bfa2e14b89877581c734c774864247e6f50400000000ffffffff01aa0a00000 \     00000001976a9146d18edfe073d53f84dd491dae1379f8fb0dfe5d488ac00000000" \     "00000000000000020000000000989680"     "1LifmeXYHeUe2qdKWBGVwfbUCMMrwYtoMm" \     "0252ce4bdd3ce38b4ebbc5a6e1343608230da508ff12d23d85b58c964204c4cef3"
func OmniCreaterawtxMultisig(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreaterawtxMultisigCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreaterawtxInput Adds a transaction input to the transaction.,If no raw transaction is provided, a new transaction is created.
// $ omnicore-cli "omni_createrawtx_input" \     "01000000000000000000" "b006729017df05eda586df9ad3f8ccfee5be340aadf88155b784d1fc0e8342ee" 0
func OmniCreaterawtxInput(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreaterawtxInputCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreaterawtxReference Adds a reference output to the transaction.,If no raw transaction is provided, a new transaction is created.,The output value is set to at least the dust threshold.
// $ omnicore-cli "omni_createrawtx_reference" \     "0100000001a7a9402ecd77f3c9f745793c9ec805bfa2e14b89877581c734c774864247e6f50400000000ffffffff03aa0a00000     00000001976a9146d18edfe073d53f84dd491dae1379f8fb0dfe5d488ac5c0d0000000000004751210252ce4bdd3ce38b4ebbc5a     6e1343608230da508ff12d23d85b58c964204c4cef3210294cc195fc096f87d0f813a337ae7e5f961b1c8a18f1f8604a909b3a51     21f065b52aeaa0a0000000000001976a914946cb2e08075bcbaf157e47bcb67eb2b2339d24288ac00000000" \     "1CE8bBr1dYZRMnpmyYsFEoexa1YoPz2mfB" \     0.005
func OmniCreaterawtxReference(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreaterawtxReferenceCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreaterawtxChange Adds a change output to the transaction.,The provided inputs are not added to the transaction, but only used to determine the change. It is assumed that the inputs were previously added, for example via `"createrawtransaction"`.,Optionally a position can be provided, where the change output should be inserted, starting with `0`. If the number of outputs is smaller than the position, then the change output is added to the end. Change outputs should be inserted before reference outputs, and as per default, the change output is added to the`first position.,If the change amount would be considered as dust, then no change output is added.
// $ omnicore-cli "omni_createrawtx_change" \     "0100000001b15ee60431ef57ec682790dec5a3c0d83a0c360633ea8308fbf6d5fc10a779670400000000ffffffff025c0d00000 \     000000047512102f3e471222bb57a7d416c82bf81c627bfcd2bdc47f36e763ae69935bba4601ece21021580b888ff56feb27f17f \     08802ebed26258c23697d6a462d43fc13b565fda2dd52aeaa0a0000000000001976a914946cb2e08075bcbaf157e47bcb67eb2b2 \     339d24288ac00000000" \     "[{\"txid\":\"6779a710fcd5f6fb0883ea3306360c3ad8c0a3c5de902768ec57ef3104e65eb1\",\"vout\":4, \     \"scriptPubKey\":\"76a9147b25205fd98d462880a3e5b0541235831ae959e588ac\",\"value\":0.00068257}]" \     "1CE8bBr1dYZRMnpmyYsFEoexa1YoPz2mfB" 0.000035 1
func OmniCreaterawtxChange(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreaterawtxChangeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadSimplesend Create the payload for a simple send transaction.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_simplesend" 1 "100.0"
func OmniCreatepayloadSimplesend(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadSimplesendCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadSendall Create the payload for a send all transaction.
// $ omnicore-cli "omni_createpayload_sendall" 2
func OmniCreatepayloadSendall(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadSendallCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadDexsell Create a payload to place, update or cancel a sell offer on the traditional distributed OMNI/BTC exchange.
// $ omnicore-cli "omni_createpayload_dexsell" 1 "1.5" "0.75" 25 "0.0005" 1
func OmniCreatepayloadDexsell(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadDexsellCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadDexaccept Create the payload for an accept offer for the specified token and amount.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_dexaccept" 1 "15.0"
func OmniCreatepayloadDexaccept(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadDexacceptCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadSto Creates the payload for a send-to-owners transaction.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_sto" 3 "5000"
func OmniCreatepayloadSto(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadStoCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadIssuancecrowdsale Creates the payload for a new tokens issuance with crowdsale.
// $ omnicore-cli "omni_createpayload_issuancecrowdsale" 2 1 0 "Companies" "Bitcoin Mining" "Quantum Miner" "" "" 2 "100" 1483228800 30 2
func OmniCreatepayloadIssuancecrowdsale(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadIssuancecrowdsaleCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadIssuancemanaged Creates the payload for a new tokens issuance with manageable supply.
// $ omnicore-cli "omni_createpayload_issuancemanaged" 2 1 0 "Companies" "Bitcoin Mining" "Quantum Miner" "" ""
func OmniCreatepayloadIssuancemanaged(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadIssuancemanagedCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadClosecrowdsale Creates the payload to manually close a crowdsale.
// $ omnicore-cli "omni_createpayload_closecrowdsale" 70
func OmniCreatepayloadClosecrowdsale(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadClosecrowdsaleCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadGrant Creates the payload to issue or grant new units of managed tokens.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_grant" 51 "7000"
func OmniCreatepayloadGrant(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadGrantCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadRevoke Creates the payload to revoke units of managed tokens.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!f
// $ omnicore-cli "omni_createpayload_revoke" 51 "100"
func OmniCreatepayloadRevoke(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadRevokeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadChangeissuer Creates the payload to change the issuer on record of the given tokens.
// $ omnicore-cli "omni_createpayload_changeissuer" 3
func OmniCreatepayloadChangeissuer(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadChangeissuerCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadTrade Creates the payload to place a trade offer on the distributed token exchange.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_trade" 31 "250.0" 1 "10.0"
func OmniCreatepayloadTrade(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadTradeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadCanceltradesbyprice Creates the payload to cancel offers on the distributed token exchange with the specified price.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_canceltradesbyprice" 31 "100.0" 1 "5.0"
func OmniCreatepayloadCanceltradesbyprice(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadCanceltradesbypriceCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadCanceltradesbypair Creates the payload to cancel all offers on the distributed token exchange with the given currency pair.
// $ omnicore-cli "omni_createpayload_canceltradesbypair" 1 31
func OmniCreatepayloadCanceltradesbypair(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadCanceltradesbypairCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadCancelalltrades Creates the payload to cancel all offers on the distributed token exchange with the given currency pair.
// $ omnicore-cli "omni_createpayload_cancelalltrades" 1
func OmniCreatepayloadCancelalltrades(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadCancelalltradesCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadEnablefreezing Creates the payload to enable address freezing for a centrally managed property.
// $ omnicore-cli "omni_createpayload_enablefreezing" 3
func OmniCreatepayloadEnablefreezing(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadEnablefreezingCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadDisablefreezing Creates the payload to disable address freezing for a centrally managed property.,IMPORTANT NOTE:  Disabling freezing for a property will UNFREEZE all frozen addresses for that property!
// $ omnicore-cli "omni_createpayload_disablefreezing" 3
func OmniCreatepayloadDisablefreezing(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadDisablefreezingCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadFreeze Creates the payload to freeze an address for a centrally managed token.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_freeze" "3HTHRxu3aSDV4deakjC7VmsiUp7c6dfbvs" 31 "100"
func OmniCreatepayloadFreeze(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadFreezeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniCreatepayloadUnfreeze Creates the payload to unfreeze an address for a centrally managed token.,Note: if the server is not synchronized, amounts are considered as divisible, even if the token may have indivisible units!
// $ omnicore-cli "omni_createpayload_unfreeze" "3HTHRxu3aSDV4deakjC7VmsiUp7c6dfbvs" 31 "100"
func OmniCreatepayloadUnfreeze(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniCreatepayloadUnfreezeCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetfeecache Obtains the current amount of fees cached (pending distribution).,If a property ID is supplied the results will be filtered to show this property ID only.  If no property ID is supplied the results will contain all properties that currently have fees cached pending distribution.
// $ omnicore-cli "omni_getfeecache" 31
func OmniGetfeecache(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetfeecacheCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetfeetrigger Obtains the amount at which cached fees will be distributed.,If a property ID is supplied the results will be filtered to show this property ID only.  If no property ID is supplied the results will contain all properties.
// $ omnicore-cli "omni_getfeetrigger" 31
func OmniGetfeetrigger(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetfeetriggerCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetfeeshare Obtains the current percentage share of fees addresses would receive if a distribution were to occur.,If an address is supplied the results will be filtered to show this address only.  If no address is supplied the results will be filtered to show wallet addresses only.  If a wildcard is provided (```"*"```) the results will contain all addresses that would receive a share.,If an ecosystem is supplied the results will reflect the fee share for that ecosystem (main or test).  If no ecosystem is supplied the results will reflect the main ecosystem.
// $ omnicore-cli "omni_getfeeshare" "1CE8bBr1dYZRMnpmyYsFEoexa1YoPz2mfB" 1
func OmniGetfeeshare(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetfeeshareCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetfeedistribution Obtains data for a past distribution of fees.,A distribution ID must be supplied to identify the distribution to obtain data for.
// $ omnicore-cli "omni_getfeedistribution" 1
func OmniGetfeedistribution(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetfeedistributionCmd)
	return omni_cmdReq(icmd, w)
}

// OmniGetfeedistributions Obtains data for past distributions of fees for a property.,A property ID must be supplied to retrieve past distributions for.
// $ omnicore-cli "omni_getfeedistributions" 31
func OmniGetfeedistributions(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniGetfeedistributionsCmd)
	return omni_cmdReq(icmd, w)
}

// OmniSetautocommit Sets the global flag that determines whether transactions are automatically committed and broadcasted.
// $ omnicore-cli "omni_setautocommit" false
func OmniSetautocommit(icmd interface{}, w *wallet.Wallet) (interface{}, error) {
	_ = icmd.(*hcjson.OmniSetautocommitCmd)
	return omni_cmdReq(icmd, w)
}
