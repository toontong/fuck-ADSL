package ctrl

import (
	"encoding/json"
	"libs/log"
)

const (
	Msg_Sys            = 0x00001000
	Msg_Sys_Ok         = 0x00001000 // 4096
	Msg_Sys_Err        = 0x00001001
	Msg_Request_Finish = 0x00001002
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

var msgString map[int64]string

func init() {
	frame := WebSocketControlFrame{
		Type:    Msg_Request_Finish,
		Index:   0,
		Content: "",
	}
	RquestFinishFrame = frame.Bytes()

	msgString = make(map[int64]string, 128)
	msgString[Msg_Sys_Ok] = "OK"
	msgString[Msg_Sys_Err] = "Err"
	msgString[Msg_Request_Finish] = "Req-finish"
}

func (this *WebSocketControlFrame) TypeS() string {
	s, ok := msgString[this.Type]
	if !ok {
		log.Warn("not found msg type[%v]", this.Type)
	}
	return s
}
