// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers 
// Copyright (c) 2018-2020 The Hc developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"testing"
)

func TestMemPool(t *testing.T) {
	pver := ProtocolVersion

	// Ensure the command is expected value.
	wantCmd := "mempool"
	msg := NewMsgMemPool()
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgMemPool: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value.
	wantPayload := uint32(0)
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Test encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BtcEncode(&buf, pver)
	if err != nil {
		t.Errorf("encode of MsgMemPool failed %v err <%v>", msg, err)
	}

	// Test decode with latest protocol version.
	readmsg := NewMsgMemPool()
	err = readmsg.BtcDecode(&buf, pver)
	if err != nil {
		t.Errorf("decode of MsgMemPool failed [%v] err <%v>", buf, err)
	}

	return
}
