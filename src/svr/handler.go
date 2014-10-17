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

type wsClient struct {
	WSocket *websocket.Conn
	Req     *http.Request // websoket 对应的 Request

	RW      *sync.RWMutex
	Working bool //是否有绑定ip-forward

	finish chan int       // 绑定连接的是否完成
	Writer io.WriteCloser // ip-forward 绑定的连接
}

type onlineHost struct {
	// 同一个HOST IP的连接集合
	MapClients map[string]*wsClient // key is IP:Port
}

type allHosts struct {
	Hosts map[string]*onlineHost //key is IP
	RW    *sync.RWMutex
}

var TypeS = websocket.MsgTypeS

// ==================
var globalAllHosts = &allHosts{
	Hosts: make(map[string]*onlineHost),
	RW:    new(sync.RWMutex),
}

func (client *wsClient) String() string {
	return client.Req.RemoteAddr
}

func (c *wsClient) handlerControlMessage(bFrame []byte) error {
	msg := ctrl.WebSocketControlFrame{}

	if err := json.Unmarshal(bFrame, &msg); err != nil {
		log.Warn("Recve a Text-Frame not JSON format. err=%v, frame=%v", err.Error(), string(bFrame))
		return err
	}
	log.Debug("TCP[%v] Get Frame T[%v], Content=%v, index=%v, ", c, msg.TypeS(), msg.Content, msg.Index)

	switch msg.Type {
	case ctrl.Msg_Request_Finish:
		c.closeWriter()
	case ctrl.Msg_Client_Busy:
		c.Writer.Write([]byte("current client was busy, pls try anthor."))
		c.closeWriter()
	case ctrl.Msg_Sys_Err:
		c.Writer.Write([]byte(msg.Content))
		c.closeWriter()
	default:
		log.Warn("no handler Msg T[%s]", msg.TypeS())
	}
	return nil
}

func (c *wsClient) closeWriter() {
	if c.Writer == nil {
		log.Warn("whan to close an nil client.Writer.")
		return
	}
	c.RW.Lock()
	defer c.RW.Unlock()
	c.Writer.Close()
	c.Working = false
}

func (c *wsClient) tellClientRequestFinish() {
	c.WSocket.WriteString(ctrl.RquestFinishFrame)
}

func (c *wsClient) tellClientNewConnection() {
	c.WSocket.WriteString(ctrl.NewConnecttion)
}

func (client *wsClient) WaitForFrameLoop() {
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
			if client.Writer == nil {
				log.Warn("client.Writer is nil.")
				continue
			}

			_, err := client.Writer.Write(bFrame)
			if err != nil {
				if err != io.EOF {
					log.Error(err.Error())
				}
				client.tellClientRequestFinish()
				client.closeWriter()
			}

		case websocket.PingMessage, websocket.PongMessage: // IE-11 会无端端发一个pong上来
			client.WSocket.Pong(bFrame)
		default:
			log.Warn("TODO: revce frame-type=%v. can not handler. content=%v", TypeS(frameType), string(bFrame))
		}
	}
}

func getFreeClient() (*wsClient, error) {
	var client *wsClient
	for ip, host := range globalAllHosts.Hosts {
		log.Info("finding in host[%s]:", ip)
		for port, cli := range host.MapClients {
			log.Info("\t finding Host:port[%s]", port)
			cli.RW.Lock()
			if !cli.Working {
				client = cli
				cli.Working = true
				cli.RW.Unlock()
				break
			}
			cli.RW.Unlock()
		}
	}
	if client == nil {
		return nil, fmt.Errorf("no free-websocket connect found.")
	}
	return client, nil
}

func BindConnection(conn net.Conn) error {
	// 与一个websocket 绑定一个连接
	client, err := getFreeClient()
	if err != nil {
		return err
	}

	client.Writer = conn.(io.WriteCloser)
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
		client.WSocket.Write(p[:n], true)
	}

	log.Info("BindConnection Request finish.")

	return nil
}

func WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("WebsocketHandler:%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
	var conn *websocket.Conn
	conn, err := websocket.Upgrade(w, r, http.Header{})
	if err != nil {
		log.Error(err.Error())
		return
	}
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	var client = &wsClient{
		WSocket: conn,
		Working: false,
		RW:      new(sync.RWMutex),
		Req:     r,
		finish:  make(chan int, 1),
	}

	var host *onlineHost
	globalAllHosts.RW.RLock()
	host, find := globalAllHosts.Hosts[r.Host]
	if !find {
		host = &onlineHost{
			MapClients: make(map[string]*wsClient),
		}
		globalAllHosts.Hosts[r.Host] = host
	}
	host.MapClients[r.RemoteAddr] = client
	globalAllHosts.RW.RUnlock()
	log.Info("Put[%s] in", client)
	client.WaitForFrameLoop()

	globalAllHosts.RW.RLock()
	delete(host.MapClients, r.RemoteAddr)
	if len(host.MapClients) == 0 {
		delete(globalAllHosts.Hosts, r.Host)
	}
	globalAllHosts.RW.RUnlock()

	log.Debug("WebsocketHandler:%s closed.", r.RemoteAddr)
}
