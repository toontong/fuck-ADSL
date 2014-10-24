/*
	websocket connect clients
*/

package svr

import (
	"ctrl"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"libs/log"
	"libs/websocket"
	"net"
	"net/http"
	"sync"
	"time"
)

var frameTypStr = websocket.MsgTypeS

type wsClient struct {
	websocket *websocket.Conn
	req       *http.Request // websoket 对应的 Request

	rw *sync.RWMutex

	working       bool           //是否有绑定ip-forward连接
	finish        chan int       // 绑定连接的是否完成
	writerForward io.WriteCloser // ip-forward 绑定的连接
}

func (client *wsClient) String() string {
	return client.req.RemoteAddr
}

func (c *wsClient) handlerControlMessage(bFrame []byte) error {
	msg := ctrl.WebSocketControlFrame{}

	if err := json.Unmarshal(bFrame, &msg); err != nil {
		log.Warn("Recve a Text-Frame not JSON format. err=%v, frame=%v", err.Error(), string(bFrame))
		return err
	}
	log.Debug("TCP[%v] Get Frame T[%v], Content=%v, index=%v, ", c, msg.TypeStr(), msg.Content, msg.Index)

	switch msg.Type {
	case ctrl.Msg_Request_Finish:
		c.closeWriter()
	case ctrl.Msg_Client_Busy:
		c.writerForward.Write([]byte("current client was busy, pls try anthor."))
		c.closeWriter()
	case ctrl.Msg_Sys_Err:
		c.writerForward.Write([]byte(msg.Content))
		c.closeWriter()
	case ctrl.Msg_Get_Config:
		log.Info("Get the Client- side local network server config[%s]", msg.Content)
		pConfig.client_conf_forward_host = msg.Content
	default:
		log.Warn("no handler Msg T[%s]", msg.TypeStr())
	}
	return nil
}

func (c *wsClient) closeWriter() {
	if c.writerForward == nil {
		log.Warn("whan to close an nil client.writerForward.")
		return
	}
	c.rw.Lock()
	defer c.rw.Unlock()
	c.writerForward.Close()
	c.working = false
}

func (c *wsClient) tellClientRequestFinish() {
	c.websocket.WriteString(ctrl.RquestFinishFrame)
}

func (c *wsClient) tellClientNewConnection() {
	c.websocket.WriteString(ctrl.NewConnecttion)
}

func (c *wsClient) tellClientNeedConfig() error {
	return c.websocket.WriteString(ctrl.GetCilentConfig)
}

func (c *wsClient) tellClientSetConfig(svr string) error {
	frame := ctrl.WebSocketControlFrame{
		Type:    ctrl.Msg_Set_Config,
		Index:   0,
		Content: svr,
	}
	return c.websocket.WriteString(frame.Bytes())
}

func (client *wsClient) waitForFrameLoop() {
	for {
		frameType, bFrame, err := client.websocket.Read()

		log.Debug("TCP[%s] recv WebSocket Frame typ=[%v] size=[%d], crc32=[%d]",
			client, frameTypStr(frameType), len(bFrame), crc32.ChecksumIEEE(bFrame))

		if err != nil {
			if err != io.ErrUnexpectedEOF {
				log.Error("TCP[%s] close Unexpected err=%v", client, err.Error())
			} else {
				log.Debug("TCP[%s] close the socket. EOF.", client)
			}
			client.closeWriter()
			return
		}

		switch frameType {
		case websocket.TextMessage:
			client.handlerControlMessage(bFrame)
		case websocket.CloseMessage:
			log.Info("TCP[%s] close Frame revced. end wait Frame loop", client)
			client.closeWriter()
			return
		case websocket.BinaryMessage:
			log.Info("TCP[%s] resv-binary: %v", client, len(bFrame))
			if client.writerForward == nil {
				log.Warn("client.writerForward is nil.")
				continue
			}

			_, err := client.writerForward.Write(bFrame)
			if err != nil {
				if err != io.EOF {
					log.Error(err.Error())
				}
				client.tellClientRequestFinish()
				client.closeWriter()
			}

		case websocket.PingMessage, websocket.PongMessage: // IE-11 会无端端发一个pong上来
			client.websocket.Pong(bFrame)
		default:
			log.Warn("TODO: revce frame-type=%v. can not handler. content=%v", frameTypStr(frameType), string(bFrame))
		}
	}
}

func getFreeClient() (*wsClient, error) {
	var client *wsClient
	for _, cli := range _OnlineClient.onlines {
		cli.rw.Lock()
		if !cli.working {
			client = cli
			cli.working = true
			cli.rw.Unlock()
			break
		}
		cli.rw.Unlock()
	}

	if client == nil {
		return nil, fmt.Errorf("no free-websocket connect found.")
	}
	return client, nil
}

func bindConnection(conn net.Conn) error {
	// 与一个websocket 绑定一个连接
	client, err := getFreeClient()
	if err != nil {
		return err
	}

	client.writerForward = conn.(io.WriteCloser)
	client.tellClientNewConnection()

	reader := conn.(io.Reader)
	p := make([]byte, 4096)

	for {
		n, err := reader.Read(p)
		if err != nil {
			if err != io.EOF {
				log.Error("Reading data from[%s] err=%s", conn.RemoteAddr(), err.Error())
			}
			client.closeWriter()
			break
		}
		client.websocket.Write(p[:n], true)
	}

	log.Info("BindConnection Request finish.")

	return nil
}

type online struct {
	onlines map[string]*wsClient // key is RemoteAddr=IP:Port
	rw      *sync.RWMutex
}

// all online websocket connection.
var _OnlineClient = &online{
	onlines: make(map[string]*wsClient),
	rw:      new(sync.RWMutex),
}

func newWebsocketClient(conn *websocket.Conn, r *http.Request) *wsClient {
	var client = &wsClient{
		websocket: conn,
		working:   false,
		rw:        new(sync.RWMutex),
		req:       r,
		finish:    make(chan int, 1),
	}

	_OnlineClient.rw.Lock()
	defer _OnlineClient.rw.Unlock()

	if cli, find := _OnlineClient.onlines[r.RemoteAddr]; find {
		log.Warn("client[%s] is working[%s].", cli.req.RemoteAddr, cli.working)
		panic("some closed client did not remove from _OnlineClient ?")
	}

	_OnlineClient.onlines[r.RemoteAddr] = client
	return client
}

func websocketClose(r *http.Request) {
	_OnlineClient.rw.RLock()
	defer _OnlineClient.rw.RUnlock()
	delete(_OnlineClient.onlines, r.RemoteAddr)
}

func WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("WebsocketHandler:%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)

	if !authOK(r) {
		log.Warn("auth fail....")
		noAuthResponse(w)
		return
	}

	var conn *websocket.Conn
	conn, err := websocket.Upgrade(w, r, http.Header{})
	if err != nil {
		log.Error(err.Error())
		return
	}
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	var client = newWebsocketClient(conn, r)

	if pConfig.client_conf_forward_host == "" {
		log.Info("pConfig.client_conf_forward_host is empty. tell Client Need the Config.")
		client.tellClientNeedConfig()
	}

	log.Info("Put[%s] into the global Connect pool.", client)

	client.waitForFrameLoop()

	websocketClose(r)

	log.Debug("WebsocketHandler:%s closed.", r.RemoteAddr)
}
