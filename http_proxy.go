package boringproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

func proxyRequest(w http.ResponseWriter, r *http.Request, tunnel Tunnel, httpClient *http.Client, address string, port int, behindProxy bool) {

	if tunnel.AuthUsername != "" || tunnel.AuthPassword != "" {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header()["WWW-Authenticate"] = []string{"Basic"}
			w.WriteHeader(401)
			return
		}

		if username != tunnel.AuthUsername || password != tunnel.AuthPassword {
			w.Header()["WWW-Authenticate"] = []string{"Basic"}
			w.WriteHeader(401)
			// TODO: should probably use a better form of rate limiting
			time.Sleep(2 * time.Second)
			return
		}
	}

	downstreamReqHeaders := r.Header.Clone()

	upstreamAddr := fmt.Sprintf("%s:%d", address, port)
	upstreamUrl := fmt.Sprintf("http://%s%s", upstreamAddr, r.URL.RequestURI())

	upstreamReq, err := http.NewRequest(r.Method, upstreamUrl, r.Body)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	// ContentLength needs to be set manually because otherwise it is
	// stripped by golang. See:
	// https://golang.org/pkg/net/http/#Request.Write
	upstreamReq.ContentLength = r.ContentLength

	upstreamReq.Header = downstreamReqHeaders

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	upstreamReq.Header["X-Forwarded-Proto"] = []string{scheme}

	upstreamReq.Header["X-Forwarded-Host"] = []string{r.Host}

	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(500)
		io.WriteString(w, errMessage)
		return
	}

	xForwardedFor := remoteHost

	if behindProxy {
		xForwardedFor := downstreamReqHeaders.Get("X-Forwarded-For")
		if xForwardedFor != "" {
			xForwardedFor = xForwardedFor + ", " + remoteHost
		}
	}

	upstreamReq.Header.Set("X-Forwarded-For", xForwardedFor)
	upstreamReq.Header.Set("Forwarded", fmt.Sprintf("for=%s", remoteHost))

	// TODO: This might need to be more generic, but using r.Host. However,
	// I think that may have security implications for things like DNS
	// rebinding attacks. Not sure.
	upstreamReq.Host = tunnel.Domain

	upstreamRes, err := httpClient.Do(upstreamReq)
	if err != nil {
		errMessage := fmt.Sprintf("%s", err)
		w.WriteHeader(502)
		io.WriteString(w, errMessage)
		return
	}
	defer upstreamRes.Body.Close()

	var forwardHeaders map[string][]string

	if r.ProtoMajor > 1 {
		forwardHeaders = stripConnectionHeaders(upstreamRes.Header)
	} else {
		forwardHeaders = upstreamRes.Header
	}

	downstreamResHeaders := w.Header()

	for k, v := range forwardHeaders {
		downstreamResHeaders[k] = v
	}

	w.WriteHeader(upstreamRes.StatusCode)
	io.Copy(w, upstreamRes.Body)
}

// Need to strip out headers that shouldn't be forwarded from HTTP/1.1 to
// HTTP/2. See https://tools.ietf.org/html/rfc7540#section-8.1.2.2
var connectionHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Connection", "Transfer-Encoding", "Upgrade",
}

func stripConnectionHeaders(headers map[string][]string) map[string][]string {
	forwardHeaders := make(map[string][]string)

	for k, v := range headers {
		if stringInArray(k, connectionHeaders) {
			continue
		}

		forwardHeaders[k] = v
	}

	return forwardHeaders
}
