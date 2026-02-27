//go:build !js

package ws

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func getConnectionOptions(
	requestHeader http.Header, tlsConfig *tls.Config,
) *websocket.Dialer {
	dialer := &websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		TLSClientConfig: tlsConfig,
		HandshakeTimeout: 10 * time.Second,
	}
	// Headers are passed directly to DialContext, not set on Dialer
	// The User-Agent header will be set when calling DialContext if not present
	return dialer
}
