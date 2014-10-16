package main

import (
	"ctrl"
	"encoding/json"
	"hash/crc32"
	"io"
	"libs/log"
	"libs/websocket"
	"net"
	"net/url"
)

const (
	SVR_HOST_IP     = "127.0.0.1:10086"
	FORWARD_HOST_IP = "10.20.151.151:8080"
)

var TypeS = websocket.MsgTypeS

type Client struct {
	WSocket   *websocket.Conn
	localConn *net.Conn
}

func (client *Client) String() string {
	return client.WSocket.String()
}

func (c *Client) Write(b []byte) (int, error) {
	err := c.WSocket.WriteBinary(b)
	return len(b), err
}

func (c *Client) RequestFinish() {
	c.WSocket.WriteString(ctrl.RquestFinishFrame)
}

func (c *Client) forward(buffIn []byte, w io.Writer) {
	conn, err := net.Dial("tcp", FORWARD_HOST_IP)

	if err != nil {
		log.Error("Connect to[%s] err=%s", FORWARD_HOST_IP, err.Error())
		goto RequestFinish
	}
	defer conn.Close()
	c.localConn = &conn

	_, err = conn.Write(buffIn)
	if err != nil {
		log.Error("Write buff to[%s] err=%s", FORWARD_HOST_IP, err.Error())
		goto RequestFinish
	}

	for {
		p := make([]byte, 4096)
		n, err := conn.Read(p)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Error("When Read err=%s", err.Error())
			w.Write([]byte(err.Error()))
			goto RequestFinish
		}
		w.Write(p[:n])
	}

RequestFinish:
	log.Info("end Forward request.")
	if err != nil {
		log.Error("forward err=%s", err.Error())
	}
	c.RequestFinish()

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
	log.Info("TCP[%v] Get Frame T[%v], Content=%v, index=%v, ", c, msg.TypeS(), msg.Content, msg.Index)

	switch msg.Type {
	case ctrl.Msg_Request_Finish:
		c.close()
	default:
		log.Warn("no handler Msg T[%s]", msg.TypeS())
	}
	return nil
}

func (client *Client) waitForCommand() {
	for {
		frameType, bFrame, err := client.WSocket.Read()
		log.Debug("TCP[%s] recv WebSocket Frame typ=[%v] size=[%d], crc32=[%d]",
			client, TypeS(frameType), len(bFrame), crc32.ChecksumIEEE(bFrame))

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

			go client.forward(bFrame, client)

		case websocket.PingMessage, websocket.PongMessage: // IE-11 会无端端发一个pong上来
			client.WSocket.Pong(bFrame)
		default:
			log.Warn("TODO: revce frame-type=%v. can not handler. content=%v", TypeS(frameType), string(bFrame))
		}
	}

}

func newConnection() {
	conn, err := net.Dial("tcp", SVR_HOST_IP)
	if err != nil {
		log.Error(err.Error())
		return
	}
	ws, _, err := websocket.NewClient(conn, &url.URL{Host: SVR_HOST_IP, Path: "/_ws4client"}, nil)
	if err != nil {
		log.Error(err.Error())
		return
	}

	ws.Ping([]byte("test"))
	msgType, msg, err := ws.Read()
	if msgType != websocket.PongMessage {
		log.Error("Unexpected frame Type[%d]", msgType)
		return
	}

	client := &Client{
		WSocket: ws,
	}
	client.waitForCommand()

	log.Error("Get Ping Return[%v]", msg)
}
func main() {
	log.SetLevel(log.LevelDebug)
	go newConnection()
	go newConnection()
	go newConnection()
	newConnection()
	println("main end .")
}
