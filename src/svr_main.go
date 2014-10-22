// server of http

package main

import (
	"ctrl"
	"flag"
	"libs/log"
	"svr"
)

const (
	//WEBSOCKET_HOST_PORT   = "0.0.0.0:8081"
	WEBSOCKET_CONTORL_URI = "/admin"
	//暂时只支持 TCP 协议的 Forward，如http,ssh
	FORWARD_listen_HOST_PORT = "0.0.0.0:8080"
)

var _Websocketlisten string
var _ForwardListtion string
var _AuthUserPassword string
var _LogLevel string

func init() {
	flag.StringVar(&_ForwardListtion, "tcp", "0.0.0.0:8080", "listen[0.0.0.0:8080] of tcp data forward.")
	flag.StringVar(&_Websocketlisten, "ws", "0.0.0.0:8081", "websocket listen host[0.0.0.0:8081]")
	flag.StringVar(&_AuthUserPassword, "auth", "", "websocket connect used auth string[username:passwrod], default is no auth.")
	flag.StringVar(&_LogLevel, "log", "warn", "log level [warn|error|debug|info], output the stdout.")
}

func main() {
	flag.Parse()

	log.Info("app start forward[%s] websocket[%s] auth[%s] log-level[%s], ",
		_ForwardListtion, _Websocketlisten, _AuthUserPassword, _LogLevel)

	log.SetLevelByName(_LogLevel)

	go svr.ListenAndIPForwardServ(_ForwardListtion)
	svr.ListenWebsocketServ(_Websocketlisten, ctrl.WEBSOCKET_CONNECT_URI, WEBSOCKET_CONTORL_URI)
}
