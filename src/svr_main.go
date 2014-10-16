// server of http

package main

import (
	"fmt"
	"io"
	"libs/log"
	"net"
	"net/http"
	"svr"
	"time"
)

const (
	HTTP_HOST_PORT = ":10086"
)

func forward() {
	// Listen on TCP port 2000 on all interfaces.
	l, err := net.Listen("tcp", ":2000")
	if err != nil {
		log.Error(err.Error())
	}
	log.Info("Listening TCP :2000")
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Error("Accept err=%s", err.Error())
		}
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c net.Conn) {
			log.Info("new connect [%s]", c.RemoteAddr())
			//MAX-REUQEST size is 4096 pre-time
			p := make([]byte, 4096)
			n, err := c.Read(p)
			if err != nil {
				err := fmt.Errorf("Read err=%s", err.Error())
				log.Error(err.Error())
				c.Write([]byte(err.Error()))
				// svr.CloseRequest(c)
			} else {
				log.Info("read n=%d, len=%d", n, len(p))
				svr.SendRequestConn(p[:n], c)

			}
			c.Close()
		}(conn)

	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("httpHandler: %s %s %s", r.Method, r.URL.RequestURI(), r.RemoteAddr)

	var buff string = fmt.Sprintf("GET %s HTTP/1.1\r\nUser-Agent:test\r\nHost: 10.20.151.151:8080\r\nAccept: */*\r\n\r\n",
		r.URL.RequestURI())

	svr.SendRequest([]byte(buff), w)
	return
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, string("Content-Type: text/html\n"))
	io.WriteString(w, fmt.Sprintf("TODO: %s %s", r.Method, r.URL.Path))
	io.WriteString(w, buff)
}

func main() {

	log.SetLevel(log.LevelDebug)

	http.HandleFunc("/_ws4client", svr.WebsocketHandler)

	http.HandleFunc("/", httpHandler)

	log.Info("Server Listen to: %v", HTTP_HOST_PORT)
	svr := &http.Server{
		Addr:           HTTP_HOST_PORT,
		Handler:        nil,
		ReadTimeout:    0 * time.Second,
		WriteTimeout:   0 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go forward()
	err := svr.ListenAndServe()

	if err != nil {
		log.Error("ListenAndServe[%v], err=[%v]", HTTP_HOST_PORT, err.Error())
	}

}
