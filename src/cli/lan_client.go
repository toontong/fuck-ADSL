package cli

import (
	"ctrl"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"libs/log"
	"libs/websocket"
	"net"
	"net/http"
	"net/url"
	"sync"
)

const (
	Default_Buffer_Size = 4096
	// chan using memory is Channel_Size * Default_Buffer_Size
	Default_Channel_Size = 4
)

var frameTypStr = websocket.MsgTypeS

type Config struct {
	LocalHostServ string `json:"LocalHostServ"`
	WebsocketAuth string `json:"WebsocketAuth"`
}

var pConfig *Config

var g_localForwardHostAndPort string

func setLocalForardHostAndPort(hostAndPort string) {
	g_localForwardHostAndPort = hostAndPort
}

type Client struct {
	WSocket     *websocket.Conn
	localConn   *net.Conn
	forwardData chan []byte // 从websock中接收到Binary数据，转发到localConn中
	rw          *sync.RWMutex
}

func NewClient(ws *websocket.Conn) *Client {
	client := &Client{
		WSocket: ws,
		rw:      new(sync.RWMutex),
	}
	return client
}

func (client *Client) String() string {
	return client.WSocket.String()
}

func (c *Client) Write(b []byte) (int, error) {
	err := c.WSocket.WriteBinary(b)
	return len(b), err
}

func (c *Client) tellServRequestFinish() {
	// tel the server this thread connect to the local-host was close or data tans finish.
	c.WSocket.WriteString(ctrl.RquestFinishFrame)
}

func (c *Client) tellServBusy() {
	// tell the server this thread was busy
	c.WSocket.WriteString(ctrl.ClientBusyFrame)
}

func (c *Client) tellServError(err error) {
	frame := ctrl.WebSocketControlFrame{
		Type:    ctrl.Msg_Sys_Err,
		Index:   0,
		Content: err.Error(),
	}
	c.WSocket.WriteString(frame.Bytes())
}

func (c *Client) connect2LoalNetwork() {
	// 与本地局域网服务器g_localForwardHostAndPort 建立socket连接
	c.rw.Lock()
	defer c.rw.Unlock()

	if c.localConn != nil {
		log.Warn("thread is busy. can not create new connect.")
		c.tellServBusy()
		return
	}

	conn, err := net.Dial("tcp", g_localForwardHostAndPort)
	if err != nil {
		log.Error("Connect to[%s] err=%s", g_localForwardHostAndPort, err.Error())
		c.tellServError(err)
		return
	}

	go c.readForward(conn)

	c.localConn = &conn
	c.forwardData = make(chan []byte, Default_Channel_Size)

	go c.writerForward(conn)

	log.Info("new connection was create [%s]", conn.RemoteAddr())
}

func (c *Client) closeLoalNetworkConnection(isReader bool) {
	c.rw.Lock()
	defer c.rw.Unlock()
	if c.localConn == nil {
		return
	}
	if isReader {
		// 如果readForward函数先结束，writerForward 中的buff = <- c.forwardData会一直阻塞
		// 此处发一个字节作信号，使 writerForward 退出
		c.forwardData <- []byte("")
	}

	(*c.localConn).Close()
	c.tellServRequestFinish()
	log.Info("connection was close[%s]", (*c.localConn).RemoteAddr())
	c.localConn = nil
}

func (c *Client) writerForward(writer io.Writer) {
	buff := make([]byte, Default_Buffer_Size)
	var err error

	for {
		buff = <-c.forwardData
		_, err = writer.Write(buff)
		if err != nil {
			break
		}
	}

	if err != nil && err != io.EOF {
		log.Error("Write to Local-network err=%s", err.Error())
	}

	log.Info("end writer forward.")
	c.closeLoalNetworkConnection(false)
}

func (c *Client) readForward(reader io.Reader) {
	// 从本地局域网连接中读取到数据，通过websocket的Binary帧方式发给服务器
	p := make([]byte, Default_Buffer_Size)
	var err error
	for {
		n, err := reader.Read(p)
		if err != nil {
			break
		}
		c.Write(p[:n])
	}

	if err != nil && err != io.EOF {
		log.Error("Write to Local-network err=%s", err.Error())
	}

	log.Info("end reader forward.")
	c.closeLoalNetworkConnection(true)
}

func (c *Client) close() {
	if c.localConn != nil {
		(*c.localConn).Close()
	}
}

func (c *Client) handlerControlFrame(bFrame []byte) error {
	msg := ctrl.WebSocketControlFrame{}

	if err := json.Unmarshal(bFrame, &msg); err != nil {
		log.Error("Recve a Text-Frame not JSON format. err=%v, frame=%v", err.Error(), string(bFrame))
		return err
	}
	log.Info("TCP[%v] Get Frame T[%v], Content=%v, index=%v, ", c, msg.TypeStr(), msg.Content, msg.Index)

	switch msg.Type {
	case ctrl.Msg_Request_Finish:
		c.close()
	case ctrl.Msg_New_Connection:
		c.connect2LoalNetwork()
	default:
		log.Warn("no handler Msg T[%s]", msg.TypeStr())
	}
	return nil
}

func (client *Client) waitForCommand() {
	for {
		frameType, bFrame, err := client.WSocket.Read()
		log.Debug("TCP[%s] recv WebSocket Frame typ=[%v] size=[%d], crc32=[%d]",
			client, frameTypStr(frameType), len(bFrame), crc32.ChecksumIEEE(bFrame))

		if err != nil {
			if err != io.ErrUnexpectedEOF {
				log.Error("TCP[%s] close Unexpected err=%v", client, err.Error())
			} else {
				log.Debug("TCP[%s] close the socket. EOF.", client)
			}
			client.close()
			return
		}

		switch frameType {
		case websocket.CloseMessage:
			log.Info("TCP[%s] close Frame revced. end wait Frame loop", client)
			client.close()
			return
		case websocket.TextMessage:
			client.handlerControlFrame(bFrame)
		case websocket.BinaryMessage:
			client.forwardData <- bFrame
			log.Debug("put frame to client.forwardData")
		case websocket.PingMessage, websocket.PongMessage: // IE-11 会无端端发一个pong上来
			client.WSocket.Pong(bFrame)
		default:
			log.Warn("TODO: revce frame-type=%v. can not handler. content=%v", frameTypStr(frameType), string(bFrame))
		}
	}
}

func Connect2Serv(forwardServ string, conf *Config) {
	if conf == nil {
		log.Error("config is nil.")
		return
	}
	// global pConfig
	pConfig = conf

	var localServ, auth = conf.LocalHostServ, conf.WebsocketAuth

	setLocalForardHostAndPort(localServ)

	websockURI := ctrl.WEBSOCKET_CONNECT_URI

	conn, err := net.Dial("tcp", forwardServ)
	if err != nil {
		log.Error(err.Error())
		return
	}
	var headers http.Header
	if auth != "" {
		headers = http.Header{}
		headers.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(auth))))
	}

	ws, _, err := websocket.NewClient(conn, &url.URL{Host: forwardServ, Path: websockURI}, headers)
	if err != nil {
		log.Error("Connect to[%s] err=%s", forwardServ, err.Error())
		return
	}

	ws.Ping([]byte("Ping"))
	msgType, _, err := ws.Read()
	if msgType != websocket.PongMessage {
		log.Error("Unexpected frame Type[%d]", msgType)
		return
	}

	client := NewClient(ws)

	log.Info("Connect[%s] websocket[%s] success, wait for server command.", serv, websockURI)
	client.waitForCommand()

	log.Info("client thread exist.")
}
