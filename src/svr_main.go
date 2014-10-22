// server of http

package main

import (
	"flag"
	"libs/log"
	"svr"
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

	var conf = &svr.Config{Auth: _AuthUserPassword}

	svr.ListenIPForwardAndWebsocketServ(_ForwardListtion, _Websocketlisten, conf)
}
