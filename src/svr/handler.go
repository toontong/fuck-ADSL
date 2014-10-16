/*
	websocket connect clients
*/

package svr

import (
	"bufio"
	"bytes"
	"ctrl"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"libs/log"
	"libs/websocket"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

type wsClient struct {
	WSocket    *websocket.Conn
	Working    bool
	RW         *sync.RWMutex
	Req        *http.Request
	RespWriter http.ResponseWriter
	finish     chan int
	Writer     io.Writer
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

func (client *wsClient) writeRespone(buff []byte) error {
	r := bufio.NewReader(bytes.NewReader(buff))
	tp := textproto.NewReader(r)

	resp := &http.Response{
		Request: nil,
	}
	// Parse the first line of the response.
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	f := strings.SplitN(line, " ", 3)
	if len(f) < 2 {
		return fmt.Errorf("malformed HTTP response[%s]", line)
	}
	reasonPhrase := ""
	if len(f) > 2 {
		reasonPhrase = f[2]
	}
	resp.Status = f[1] + " " + reasonPhrase
	resp.StatusCode, err = strconv.Atoi(f[1])
	if err != nil {
		return fmt.Errorf("malformed HTTP status code[%s]", f[1])
	}

	resp.Proto = f[0]
	var ok bool
	if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
		return fmt.Errorf("malformed HTTP version[%s]", resp.Proto)
	}

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	respWriter := client.RespWriter
	for key, values := range mimeHeader {
		for _, val := range values {
			log.Info("Add header %s:%s", key, val)
			respWriter.Header().Set(key, val)
		}
	}

	// send the header and status-code
	respWriter.WriteHeader(resp.StatusCode)
	for {
		// send the body
		p := make([]byte, 4096)
		n, err := r.Read(p)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Error("Read body err=%s", err.Error())
			return err
		} else {
			log.Info("Send body-size=%d", n)
			_, err := respWriter.Write(p)
			if err != nil {
				log.Error("Send body err=%s", err.Error())
				return err
			}
		}
	}

	return nil
}

func (c *wsClient) handlerControlMessage(bFrame []byte) error {
	msg := ctrl.WebSocketControlFrame{}

	if err := json.Unmarshal(bFrame, &msg); err != nil {
		log.Warn("Recve a Text-Frame not JSON format. err=%v, frame=%v", err.Error(), string(bFrame))
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

func (c *wsClient) close() {
	if c.Working {
		c.finish <- 1
	}
	c.Working = false
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
			client.close()
			return
		}

		switch frameType {
		case websocket.TextMessage:
			client.handlerControlMessage(bFrame)
		case websocket.CloseMessage:
			log.Info("TCP[%s] close Frame revced. end wait Frame loop", client)
			client.close()
			return
		case websocket.BinaryMessage:
			log.Info("TCP[%s] resv-binary: %v", client, len(bFrame))

			_, err := client.Writer.Write(bFrame)
			if err != nil {
				log.Error(err.Error())
				client.WSocket.WriteString(ctrl.RquestFinishFrame)
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

func SendRequestConn(buffer []byte, w io.Writer) error {
	client, err := getFreeClient()
	if err != nil {
		return err
	}
	log.Info("send request to -> TCP[%s] - %s", client, buffer)
	client.Writer = w
	client.WSocket.Write(buffer, true)

	var finish int
	finish = <-client.finish
	client.Working = false
	log.Info("Request finish[%d].", finish)

	return nil
}

func SendRequest(buffer []byte, w http.ResponseWriter) error {
	client, err := getFreeClient()
	if err != nil {
		return err
	}

	log.Info("send request to -> TCP[%s] - %s", client, buffer)
	client.RespWriter = w
	client.Writer = w
	client.WSocket.Write(buffer, true)

	var finish int
	finish = <-client.finish
	client.Working = false
	log.Info("Request finish[%d].", finish)
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
