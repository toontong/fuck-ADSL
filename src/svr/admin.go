package svr

import (
	"io"
	"libs/log"
	"net/http"
)

func HttpAdminHandler(w http.ResponseWriter, r *http.Request) {
	//　TODO：　做web控制后台
	log.Info("httpHandler: %s %s %s", r.Method, r.URL.RequestURI(), r.RemoteAddr)

	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, string("Content-Type: text/html\n"))
	io.WriteString(w, "TODO: 留做web控制后台")
}
