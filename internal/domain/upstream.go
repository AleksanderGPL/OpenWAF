package domain

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func ValidHostname(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}

func UpstreamURL(value string) (*url.URL, error) {
	target, err := url.Parse(value)
	if err != nil || target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" || target.User != nil || (target.Path != "" && target.Path != "/") || target.RawQuery != "" || target.ForceQuery || target.Fragment != "" || !ValidHostname(strings.ToLower(target.Hostname())) {
		return nil, errors.New("Upstream URL must be an HTTP or HTTPS origin without credentials, a path, query, or fragment")
	}
	if port := target.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, errors.New("Upstream port must be between 1 and 65535")
		}
	}
	if strings.HasSuffix(target.Host, ":") {
		return nil, errors.New("Upstream port cannot be empty")
	}
	target.Path = ""
	return target, nil
}
