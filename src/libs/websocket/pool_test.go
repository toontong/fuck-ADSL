package websocket

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func runTestServer(t *testing.T) {

	pool := NewPool()

	http.HandleFunc("/testpool/server", func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err.Error())
		}

		pool.Run(conn)
	})

	go http.ListenAndServe(":65501", nil)

	go func() {
		for {
			messageType, message, err := pool.ReadMessage()

			if err == ErrDisconnected {
				<-time.After(time.Millisecond * 10)
				continue
			}

			if err != nil {
				// disconnected one
				continue
			}

			err = pool.WriteMessage(messageType, message)

			if err != nil {
				//println("SERVER: write error:", err.Error())
			}
		}
	}()
}

func connToServer() (*Conn, error) {
	conn, err := net.Dial("tcp", "127.0.0.1:65501")
	if err != nil {
		return nil, err
	}
	ws, _, err := NewClient(conn, &url.URL{Host: "127.0.0.1:65501", Path: "/testpool/server"}, nil)
	return ws, err
}

func packData(id int, msg []byte) []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, uint32(id))
	buf.Write(msg)
	return buf.Bytes()
}

func unpackData(data []byte) (id int, msg []byte) {
	id = int(binary.BigEndian.Uint32(data[:4]))
	msg = data[4:]
	return id, msg
}

func runWorker(id int, ch chan []byte, cliPool *Pool, t *testing.T) {
	for i := 0; i < 3; i++ {
		msg := make([]byte, 50)
		rand.Read(msg)
		data := packData(id, msg)
		err := cliPool.WriteMessage(BinaryMessage, data)
		if err != nil {
			//println("CLIENT: write err:", err.Error())
			continue
		}

		select {
		case <-time.After(time.Millisecond * 200):
			t.Error("ERROR: timeout", id)
			return
		case result := <-ch:
			if bytes.Compare(msg, result) != 0 {
				t.Error("ERROR: compare failed")
			}
			//println("CLIENT: test OK. ----------------", id)
		}

		<-time.After(time.Millisecond * 300)
	}
}

func runClient(t *testing.T) {
	connCount := 10
	workerCount := 10

	cliPool := NewPool()

	conns := make([]*Conn, 0)
	for i := 0; i < connCount; i++ {
		c, err := connToServer()
		if err != nil {
			t.Fatal(err.Error())
		}

		go cliPool.Run(c)

		conns = append(conns, c)
	}

	// run test

	reqMap := make(map[int]chan []byte)
	for i := 0; i < workerCount; i++ {
		reqMap[i] = make(chan []byte)
	}

	for i := 0; i < workerCount; i++ {
		go runWorker(i, reqMap[i], cliPool, t)
	}

	go func() {
		for {
			msgType, data, err := cliPool.ReadMessage()

			if err == ErrDisconnected {
				//println("CLIENT: ALL Connections disconnected.")
				break
			}
			if err != nil {
				//println("CLIENT: read err", err.Error())
				continue
			}
			if msgType == CloseMessage {
				//println("CLIENT: read close msg")
				continue
			}

			id, msg := unpackData(data)

			if ch, ok := reqMap[id]; ok {
				ch <- msg
			}
		}
	}()

	<-time.After(time.Second)

	for _, c := range conns {
		c.Close()
	}
}

func TestWebsocketPool(t *testing.T) {

	println("Start run server")
	runTestServer(t)

	println("Start run client")
	runClient(t)
}
