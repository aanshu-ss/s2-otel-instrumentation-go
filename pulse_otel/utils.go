package pulse_otel

import (
	"net"
	"net/url"
	"time"
)

// isReachable checks if the host:port endpoint is reachable within the timeout
func isReachable(endpoint string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func stripScheme(fullURL string) (string, error) {
	parsed, err := url.Parse(fullURL)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}
