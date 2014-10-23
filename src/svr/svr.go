package svr

import (
	"ctrl"
	"encoding/base64"
	"fmt"
	"libs/log"
	"net"
	"net/http"
	"time"
)

const (
	WEBSOCKET_CONTORL_URI = "/admin"
)

type Config struct {
	Auth                     string // username:password, used in the http-header()
	Stop                     bool   // TODO
	client_conf_forward_host string // client's local network servier ip:host which data forward
}

var pConfig *Config

func authOK(req *http.Request) bool {
	if pConfig == nil && pConfig.Auth == "" {
		log.Info("did not need auth.")
		return true
	}

	var auth = req.Header.Get("Authorization")
	log.Debug("reuquest Auth->[%s]", auth)

	return auth == pConfig.Auth
}

func noAuthResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("WWW-Authenticate", "Basic realm=\"TCP-Forward-Serv\"")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, pre-check=0, post-check=0, max-age=0")
	w.Header().Set("Expires", "Mon, 3 Jan 2000 12:34:56 GMT")
	w.WriteHeader(401)
	w.Write([]byte(`401: Not Authenticated!username and password do not match to configuration`))
}

func ipforward(c net.Conn) {
	//只支持 TCP 协议的 Forward，如http,ssh
	log.Debug("new connect [%s]", c.RemoteAddr())
	defer c.Close()
	err := bindConnection(c)

	if err != nil {
		log.Error("Forward PutConnection err=%s", err.Error())
		c.Write([]byte(err.Error())) //c maybe closed.
	}
}

func ListenIPForwardAndWebsocketServ(forwardHostAndPort, websocketHostAndPort string, conf *Config) {
	if conf == nil {
		panic("config is nil.")
	}
	pConfig = conf
	if pConfig.Auth != "" {
		pConfig.Auth = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(pConfig.Auth)))
	}

	go listenWebsocketServ(websocketHostAndPort, ctrl.WEBSOCKET_CONNECT_URI, WEBSOCKET_CONTORL_URI)

	l, err := net.Listen("tcp", forwardHostAndPort)
	if err != nil {
		log.Error(err.Error())
	}
	defer l.Close()

	log.Debug("IP Forward Listening TCP[%s]", forwardHostAndPort)

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

func listenWebsocketServ(hostAndPort string, websocketURI, contorlURI string) {
	// TODO：连接认证
	http.HandleFunc(websocketURI, WebsocketHandler)

	http.HandleFunc(contorlURI, HttpAdminHandler)

	log.Info("Websocket Listen in TCP[%s]", hostAndPort)
	svr := &http.Server{
		Addr:           hostAndPort,
		Handler:        nil,
		ReadTimeout:    0 * time.Second,
		WriteTimeout:   0 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1M
	}

	err := svr.ListenAndServe()

	if err != nil {
		log.Error("ListenAndServe[%v], err=[%v]", hostAndPort, err.Error())
	}
	log.Info("ListenWebsocketServ exit.")
}
