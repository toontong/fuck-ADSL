package main

import (
	"cli"
	"flag"
	"libs/log"
)

const (
	//WEBSOCKET_SVR_HOST_IP         = "127.0.0.1:8081"
	//LOCAL_NETWORK_FORWARD_HOST_IP = "10.20.151.151:8080"
	MIN_THREAD = 4
)

var globalForwardServer string
var globalAuthUserPassword string
var globalForwardTHread int

// read from the host and port in the local-network
var globalLocalNetworkHost string

var globalLogLevel string

func init() {
	flag.StringVar(&globalForwardServer, "for", "114.114.114.114:8081", "websocket connect to [14.114.114.114:8080] for TCP-data forward.")
	flag.StringVar(&globalAuthUserPassword, "auth", "", "websocket connect used auth string[username:passwrod], default is no auth.")
	flag.StringVar(&globalLocalNetworkHost, "host", "192.168.1.101:8080", "local-network host which can not listion WLAN-IP.")
	flag.StringVar(&globalLogLevel, "log", "warn", "log level [warn|error|debug|info], output the stdout.")
	flag.IntVar(&globalForwardTHread, "n", MIN_THREAD, "conut of the thread which read for local-host to the forward-server, min is 4.")
}

func main() {
	flag.Parse()
	log.Info("start app, Read From[%s], forward data To[%s], auth[%s] log-level[%s].",
		globalLocalNetworkHost, globalForwardServer, globalAuthUserPassword, globalLogLevel)

	log.SetLevelByName(globalLogLevel)

	if globalForwardTHread < MIN_THREAD {
		globalForwardTHread = MIN_THREAD
	}

	cli.SetLocalForardHostAndPort(globalLocalNetworkHost)
	for i := 0; i < globalForwardTHread-1; i++ {
		go cli.Connect2Serv(globalForwardServer)
	}
	cli.Connect2Serv(globalForwardServer)

	log.Info("main end .")
}
