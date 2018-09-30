package omnilib

import "time"



func OmniCommunicate(netName string) {
	//add by ycj 20180915
	LoadLibAndInit()
	OmniStart(netName)

	time.Sleep(time.Second * 2)
	/*
		strReq := "{\"method\":\"omni_getinfo\",\"params\":[],\"id\":1}\n"
		strRsp := JsonCmdReqHcToOm(strReq)
		fmt.Println("in Go strRsp 1:", strRsp)
	*/

}

type Request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}
