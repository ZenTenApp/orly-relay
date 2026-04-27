package app

import (
	"net/http"
	"strings"

	"git.smesh.lol/orly/pkg/lol/log"
)

// GetRemoteFromReq retrieves the originating IP address of the client from
// an HTTP request, considering standard and non-standard proxy headers.
//
// # Parameters
//
//   - r: The HTTP request object containing details of the client and
//     routing information.
//
// # Return Values
//
//   - rr: A string value representing the IP address of the originating
//     remote client.
//
// # Expected behaviour
//
// The function first checks for the standardized "Forwarded" header (RFC 7239)
// to identify the original client IP. If that isn't available, it falls back to
// the "X-Forwarded-For" header. If both headers are absent, it defaults to
// using the request's RemoteAddr.
//
// For the "Forwarded" header, it extracts the client IP from the "for"
// parameter. For the "X-Forwarded-For" header, if it contains one IP, it
// returns that. If it contains two IPs, it returns the second.
func GetRemoteFromReq(r *http.Request) (rr string) {
	// First check for the standardized Forwarded header (RFC 7239)
	forwarded := r.Header.Get("Forwarded")
	if forwarded != "" {
		// Parse the Forwarded header which can contain multiple parameters
		//
		// Format:
		//
		// 	Forwarded: by=<identifier>;for=<identifier>;host=<host>;proto=<http|https>
		parts := strings.Split(forwarded, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "for=") {
				// Extract the client IP from the "for" parameter
				forValue := strings.TrimPrefix(part, "for=")
				// Remove quotes if present
				forValue = strings.Trim(forValue, "\"")
				// Handle IPv6 addresses which are enclosed in square brackets
				forValue = strings.Trim(forValue, "[]")
				return forValue
			}
		}
	}
	// If the Forwarded header is not available or doesn't contain "for"
	// parameter, fall back to X-Forwarded-For
	rem := r.Header.Get("X-Forwarded-For")
	if rem == "" {
		rr = r.RemoteAddr
	} else {
		// X-Forwarded-For can be comma-separated or space-separated;
		// use the first (client) IP for comma-separated, or handle
		// space-separated format from some proxies.
		parts := strings.Split(rem, ",")
		if len(parts) > 1 {
			rr = strings.TrimSpace(parts[0])
		} else {
			splitted := strings.Split(rem, " ")
			rr = strings.TrimSpace(splitted[0])
		}
	}
	return
}

// LogProxyInfo logs comprehensive proxy information for debugging
func LogProxyInfo(r *http.Request, prefix string) {
	proxyHeaders := map[string]string{
		"X-Forwarded-For":   r.Header.Get("X-Forwarded-For"),
		"X-Real-IP":         r.Header.Get("X-Real-IP"),
		"X-Forwarded-Proto": r.Header.Get("X-Forwarded-Proto"),
		"X-Forwarded-Host":  r.Header.Get("X-Forwarded-Host"),
		"X-Forwarded-Port":  r.Header.Get("X-Forwarded-Port"),
		"Forwarded":         r.Header.Get("Forwarded"),
		"Host":              r.Header.Get("Host"),
		"User-Agent":        r.Header.Get("User-Agent"),
	}

	var info []string
	for header, value := range proxyHeaders {
		if value != "" {
			info = append(info, header+":"+value)
		}
	}

	if len(info) > 0 {
		log.T.F("%s proxy info: %s", prefix, strings.Join(info, " "))
	}
}
