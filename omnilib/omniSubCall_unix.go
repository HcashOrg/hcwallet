// +build linux aix darwin dragonfly freebsd  netbsd openbsd solaris

package omnilib

// #include <stdio.h>
// #include <stdlib.h>
// #include "./omniproxy.h"
// #cgo CFLAGS: -I./
//#cgo LDFLAGS:-L./ -lomnicored -lbitcoin_server -lbitcoin_common -lunivalue -lbitcoin_util -lbitcoin_wallet  -lbitcoin_consensus -lbitcoin_crypto -lleveldb -lmemenv -lsecp256k1 /usr/lib/x86_64-linux-gnu/libboost_system.a /usr/lib/x86_64-linux-gnu/libboost_filesystem.a /usr/lib/x86_64-linux-gnu/libboost_program_options.a /usr/lib/x86_64-linux-gnu/libboost_thread.a /usr/lib/x86_64-linux-gnu/libboost_chrono.a /usr/lib/x86_64-linux-gnu/libdb_cxx.a /usr/lib/x86_64-linux-gnu/libssl.a /usr/lib/x86_64-linux-gnu/libcrypto.a  /usr/lib/x86_64-linux-gnu/libevent_pthreads.a /usr/lib/x86_64-linux-gnu/libevent.a -lm -ldl -lstdc++
import "C"
import (
	"unsafe"
	"fmt"

	"sync"
	"time"
)

var mutexOmni sync.Mutex

func JsonCmdReqHcToOm(strReq string) string {
	mutexOmni.Lock()
	defer mutexOmni.Unlock()
	strRsp := C.GoString(C.CJsonCmdReq(C.CString(strReq)))
	return strRsp
}

func LoadLibAndInit() {
	C.CLoadLibAndInit()
}

func OmniStart(strArgs string) {
	C.COmniStart(C.CString(strArgs))
}


var ChanReqOmToHc=make(chan string )
var ChanRspOmToHc=make(chan string )

// callback to LegacyRPC.Server
//var PtrLegacyRPCServer *Server=nil

//export JsonCmdReqOmToHc
func JsonCmdReqOmToHc(pcReq *C.char) *C.char {
	strReq:=C.GoString(pcReq)
	fmt.Println("Go JsonCmdReqOmToHc strReq=",strReq)
	ChanReqOmToHc<-strReq
	strRsp:=<-ChanRspOmToHc
	fmt.Println("Go JsonCmdReqOmToHc strRsp=",strRsp)
	cs := C.CString(strRsp)

	defer func(){
		go func() {
			time.Sleep(time.Microsecond*200)
			C.free(unsafe.Pointer(cs))
		}()
	}()

	return cs
}