package main

import (
	"cli"
	"flag"
	"libs/log"
	"os"
	"os/signal"
	"time"
)

const (
	MIN_THREAD = 8
)

var _ForwardServer string
var _AuthUserPassword string
var _ForwardTHread int

// read from the host and port in the local-network
var _LocalNetworkHost string

var _LogLevel string

func init() {
	flag.StringVar(&_ForwardServer, "f", "114.114.114.114:8081", "websocket connect to [14.114.114.114:8081] for TCP-data forward.")
	flag.StringVar(&_AuthUserPassword, "auth", "", "websocket connect used auth string[username:passwrod], default is no auth.")
	flag.StringVar(&_LocalNetworkHost, "l", "127.0.0.1:8000", "local-network host which can not listen WLAN-IP.")
	flag.StringVar(&_LogLevel, "log", "warn", "log level [warn|error|debug|info], output the stdout.")
	flag.IntVar(&_ForwardTHread, "n", MIN_THREAD, "conut of the thread which read for local-host to the forward-server, min is 8.")
}

var stop bool

func onSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	<-c
	stop = true
}

func main() {
	flag.Parse()
	log.Info("start app, Read From[%s], forward data To[%s], auth[%s] log-level[%s].",
		_LocalNetworkHost, _ForwardServer, _AuthUserPassword, _LogLevel)

	log.SetLevelByName(_LogLevel)

	if _ForwardTHread < MIN_THREAD {
		_ForwardTHread = MIN_THREAD
	}

	conf := &cli.Config{
		LocalHostServ: _LocalNetworkHost,
		WebsocketAuth: _AuthUserPassword,
		MaxThread:     _ForwardTHread,
	}
	const (
		default_sleep_time = 10 * time.Second
	)
	for {
		cli.Connect2Serv(_ForwardServer, conf)
		if stop {
			break
		} else {
			log.Info("Connect2Serv fail, wait[%s]Second.", default_sleep_time)
			time.Sleep(default_sleep_time)
		}
	}
	log.Info("------------------- main end ------------------------")
}
