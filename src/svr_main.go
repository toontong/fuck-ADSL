// server of http

package main

import (
	"ctrl"
	"flag"
	"libs/log"
	"net"
	"net/http"
	"svr"
	"time"
)

const (
	//WEBSOCKET_HOST_PORT   = "0.0.0.0:8081"
	WEBSOCKET_CONTORL_URI = "/admin"
	//暂时只支持 TCP 协议的 Forward，如http,ssh
	FORWARD_LISTION_HOST_PORT = "0.0.0.0:8080"
)

func ipforward(c net.Conn) {
	//只支持 TCP 协议的 Forward，如http,ssh
	log.Info("new connect [%s]", c.RemoteAddr())
	defer c.Close()
	err := svr.BindConnection(c)

	if err != nil {
		log.Error("Forward PutConnection err=%s", err.Error())
		c.Write([]byte(err.Error()))
	}
}

func ListenAndIPForwardServ(hostAndPort string) {
	// Listen on TCP port 2000 on all interfaces.
	l, err := net.Listen("tcp", hostAndPort)
	if err != nil {
		log.Error(err.Error())
	}
	log.Info("IP Forward Listening TCP[%s]", hostAndPort)
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Error("IP-Forward Accept err=%s", err.Error())
			continue
		}
		// handle socket data recv and send
		go ipforward(conn)
	}
	log.Info("ListenAndIPForwardServ exit.")
}

func ListenWebsocketServ(hostAndPort string, websocketURI, contorlURI string) {
	// TODO：连接认证
	http.HandleFunc(websocketURI, svr.WebsocketHandler)

	http.HandleFunc(contorlURI, svr.HttpAdminHandler)

	log.Info("Websocket Listen in TCP[%s]", hostAndPort)
	svr := &http.Server{
		Addr:           hostAndPort,
		Handler:        nil,
		ReadTimeout:    0 * time.Second,
		WriteTimeout:   0 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	err := svr.ListenAndServe()

	if err != nil {
		log.Error("ListenAndServe[%v], err=[%v]", hostAndPort, err.Error())
	}
	log.Info("ListenWebsocketServ exit.")
}

var globalWebsocketListion string
var globalForwardListtion string
var globalAuthUserPassword string
var globalLogLevel string

func init() {
	flag.StringVar(&globalForwardListtion, "tcp", "0.0.0.0:8080", "listion[0.0.0.0:8080] of tcp data forward.")
	flag.StringVar(&globalWebsocketListion, "ws", "0.0.0.0:8081", "websocket listion host[0.0.0.0:8081]")
	flag.StringVar(&globalAuthUserPassword, "auth", "", "websocket connect used auth string[username:passwrod], default is no auth.")
	flag.StringVar(&globalLogLevel, "log", "warn", "log level [warn|error|debug|info], output the stdout.")
}

func main() {
	flag.Parse()

	log.Info("app start forward[%s] websocket[%s] auth[%s] log-level[%s], ",
		globalForwardListtion, globalWebsocketListion, globalAuthUserPassword, globalLogLevel)

	log.SetLevelByName(globalLogLevel)

	go ListenAndIPForwardServ(globalForwardListtion)
	ListenWebsocketServ(globalWebsocketListion, ctrl.WEBSOCKET_CONNECT_URI, WEBSOCKET_CONTORL_URI)
}
