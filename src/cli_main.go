package main

import (
	"cli"
	"libs/log"
	"libs/websocket"
	"net"
	"net/url"
)

const (
	SVR_HOST_IP         = "127.0.0.1:8081"
	WEBSOCK_CONNECT_URI = "/_ws4client"

	FORWARD_HOST_IP = "10.20.151.151:8080"
	MIN_THREAD      = 4
)

func connect2Serv(serv, websockURI string) {
	conn, err := net.Dial("tcp", serv)
	if err != nil {
		log.Error(err.Error())
		return
	}

	ws, _, err := websocket.NewClient(conn, &url.URL{Host: serv, Path: websockURI}, nil)
	if err != nil {
		log.Error("Connect to[%s] err=%s", serv, err.Error())
		return
	}

	ws.Ping([]byte("test"))
	msgType, _, err := ws.Read()
	if msgType != websocket.PongMessage {
		log.Error("Unexpected frame Type[%d]", msgType)
		return
	}

	client := cli.NewClient(ws)

	log.Info("Connect[%s] websocket[%s] success, wait for server command.", serv, websockURI)
	client.WaitForCommand()

	log.Info("client thread exist.")
}

func main() {

	log.SetLevel(log.LevelDebug)

	cli.SetLocalForardHostAndPort(FORWARD_HOST_IP)

	for i := 0; i < MIN_THREAD-1; i++ {
		go connect2Serv(SVR_HOST_IP, WEBSOCK_CONNECT_URI)
	}
	connect2Serv(SVR_HOST_IP, WEBSOCK_CONNECT_URI)

	log.Info("main end .")
}
