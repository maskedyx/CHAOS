package validation

import (
	"net"
	"net/url"
	"regexp"
)

func IsValidIPAddress(s string) bool {
	return net.ParseIP(s) != nil
}

func IsValidURL(s string) bool {
	if _, err := url.ParseRequestURI(s); err != nil {
		return false
	}
	return true
}

func IsValidPort(port string) bool {
	match, err := regexp.MatchString("^([1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$", port)
	return match && err == nil
}

func IsValidMacAddress(address string) bool {
	match, err := regexp.MatchString("^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$", address)
	return match && err == nil
}

// IsValidHostname accepts a bare hostname such as localhost or agent.example.com.
func IsValidHostname(hostname string) bool {
	match, err := regexp.MatchString("^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$", hostname)
	return match && err == nil
}
