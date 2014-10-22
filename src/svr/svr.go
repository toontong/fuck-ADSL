package svr

import (
	"libs/log"
	"net"
	"net/http"
	"time"
)

func ipforward(c net.Conn) {
	//只支持 TCP 协议的 Forward，如http,ssh
	log.Info("new connect [%s]", c.RemoteAddr())
	defer c.Close()
	err := bindConnection(c)

	if err != nil {
		log.Error("Forward PutConnection err=%s", err.Error())
		c.Write([]byte(err.Error())) //c maybe closed.
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
	http.HandleFunc(websocketURI, WebsocketHandler)

	http.HandleFunc(contorlURI, HttpAdminHandler)

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
