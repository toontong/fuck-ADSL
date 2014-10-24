package svr

import (
	"fmt"
	"io"
	"libs/log"
	"net/http"
)

func init() {
	println("svr.admin.init")
}

var control_html = `<html><head></head><body>
localhost-IP：<input id='local_network_ip' value='%s' />
<input type='button' value='Set' onclick='javascript:setLocalnetworkHostPort();'><br>

<br><br>Note: If you find that localhost-IP does not update, try and disable your browser's cache
<lable  id='resutl'> </ lable>
<script type='text/javascript' language='javascript'>
 var http_request = false;
 //var control_port = %d 
 function setLocalnetworkHostPort(){
 	var ip = document.getElementById('local_network_ip').value 
 	makeRequest('svr='+ip)
 }
 function makeRequest(parameters) {
 	http_request = false;
  	if (window.XMLHttpRequest) { // Mozilla, Safari,...
    		http_request = new XMLHttpRequest();
    		if (http_request.overrideMimeType) {
 			// set type accordingly to anticipated content type
 			//http_request.overrideMimeType('text/xml');
 			http_request.overrideMimeType('text/html');
    		}
  	} else if (window.ActiveXObject) { // IE
  		try {
    			http_request = new ActiveXObject('Msxml2.XMLHTTP');
  		} catch (e) {
    			try {
       			http_request = new ActiveXObject('Microsoft.XMLHTTP');
  			} catch (e) {}
 		}
  	}
  	if (!http_request) {
 		alert('Cannot create XMLHTTP instance');
 		return false;
  	}
    http_request.onreadystatechange = alertContents;
	//http_request.open('GET', 'http://'+window.location.hostname+':'+control_port+'/?' + parameters, true);
	http_request.open('GET', '/api/set?' + parameters, true);
    http_request.send(null);
 }
 function alertContents() {
    if (http_request.readyState == 4) {
      if (http_request.status == 200) {
        alert(http_request.responseText);
        result = http_request.responseText;
        document.getElementById('result').innerHTML = result;            
      } else {
        alert('There was a problem controlling the camera.');
      }
    }
 }
</script>`

// URI: /admin/
func httpAdminHandler(w http.ResponseWriter, r *http.Request) {

	log.Info("admin Handler: %s %s %s", r.Method, r.URL.RequestURI(), r.RemoteAddr)

	if !authOK(r) {
		noAuthResponse(w)
		return
	}

	setSTDheader(w)
	resp := fmt.Sprintf(control_html, pConfig.client_conf_forward_host)
	io.WriteString(w, resp)
}

// URI: /api/
func httpApiHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("api Handler: %s %s %s", r.Method, r.URL.RequestURI(), r.RemoteAddr, r.FormValue("svr"))

	if !authOK(r) {
		noAuthResponse(w)
		return
	}
	var status = 200
	var resp string

	svr := r.FormValue("svr")
	if svr != "" && svr != pConfig.client_conf_forward_host {
		cli, err := getFreeClient()
		if cli == nil {
			resp = err.Error()
		} else {
			err := cli.tellClientSetConfig(svr)
			if err != nil {
				resp = err.Error()
			} else {
				resp = fmt.Sprintf("Set [%s] OK", svr)
			}
		}
	}

	setSTDheader(w)
	w.WriteHeader(status)
	io.WriteString(w, resp)
}
