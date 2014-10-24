package ctrl

import (
	"encoding/json"
	"libs/log"
)

const (
	WEBSOCKET_CONNECT_URI = "/_ws4client"
)

const (
	Msg_Sys            = 0x00001000
	Msg_Sys_Ok         = 0x00001000 // 4096
	Msg_Sys_Err        = 0x00001001
	Msg_New_Connection = 0x00001002
	Msg_Request_Finish = 0x00001003
	Msg_Client_Busy    = 0x00001004
	Msg_Get_Config     = 0x00001005
	Msg_Set_Config     = 0x00001006
)

//使用 websocket Text-Frame作为控制流。每Frame都是JSON格式
// 形如 {"type":<int>,"index":<int>,"c":"string-content"}
type WebSocketControlFrame struct {
	Type  int64 `json:"type"`
	Index int64 `json:"index"`
	// Content可转成javascript对像的JSON串, 会因type不同而转出来的结构不同
	Content string `json:"c"`
}

func (ctrl *WebSocketControlFrame) String() string {
	return string(ctrl.Bytes())
}

func (ctrl *WebSocketControlFrame) Bytes() []byte {
	bytes, err := json.Marshal(ctrl)
	if err != nil {
		panic(err.Error())
		log.Error("JSON format err=", err.Error())
		return []byte("")
	}
	return bytes
}

var RquestFinishFrame []byte
var ClientBusyFrame []byte
var NewConnecttion []byte
var GetCilentConfig []byte

var msgString map[int64]string

func init() {
	frame := WebSocketControlFrame{
		Type:    Msg_Request_Finish,
		Index:   0,
		Content: "",
	}
	RquestFinishFrame = frame.Bytes()

	frame.Type = Msg_Client_Busy
	ClientBusyFrame = frame.Bytes()

	frame.Type = Msg_New_Connection
	NewConnecttion = frame.Bytes()

	frame.Type = Msg_Get_Config
	GetCilentConfig = frame.Bytes()

	msgString = make(map[int64]string, 128)
	msgString[Msg_Sys_Ok] = "OK"
	msgString[Msg_Sys_Err] = "Err"
	msgString[Msg_New_Connection] = "New-Conn"
	msgString[Msg_Request_Finish] = "Req-finish"
	msgString[Msg_Client_Busy] = "Cli-Busy"
	msgString[Msg_Get_Config] = "Get-Conf"
	msgString[Msg_Set_Config] = "Set-Conf"
}

func (this *WebSocketControlFrame) TypeStr() string {
	s, ok := msgString[this.Type]
	if !ok {
		log.Warn("not found msg type[%v]", this.Type)
	}
	return s
}
