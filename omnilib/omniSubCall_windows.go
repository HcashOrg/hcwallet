// +build windows

package omnilib

// #include <stdio.h>
// #include <stdlib.h>
// #include "./omniproxy.h"
// #cgo CFLAGS: -I./
import "C"
import (
	//"unsafe"
	//"time"
	"fmt"
	"sync"
	"time"
	"unsafe"
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



var ChanReqOmToHc = make(chan string)
var ChanRspOmToHc = make(chan string)

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
