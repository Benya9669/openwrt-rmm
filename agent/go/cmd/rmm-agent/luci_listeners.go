package main

import (
	"net"
	"strconv"
	"strings"
)

type luciListener struct {
	host string
	port int
}

// Pick an address owned by this router, preferring IPv4 loopback on dual-stack devices.
func selectLuCIListener(bindings string, localIPs map[string]bool) luciListener {
	var selected luciListener
	for _, binding := range strings.Fields(bindings) {
		host, portText, err := net.SplitHostPort(binding)
		if err != nil {
			host, portText = "0.0.0.0", binding
		}
		port, err := strconv.Atoi(portText)
		ip := net.ParseIP(host)
		if err != nil || port < 1 || port > 65535 || ip == nil {
			continue
		}
		switch {
		case ip.IsUnspecified() && ip.To4() != nil:
			host = "127.0.0.1"
		case ip.IsUnspecified():
			host = "::1"
		case !ip.IsLoopback() && !localIPs[ip.String()]:
			continue
		}
		if host == "127.0.0.1" {
			return luciListener{host, port}
		}
		if selected.port == 0 {
			selected = luciListener{host, port}
		}
	}
	return selected
}

func configuredLuCIListeners() map[string]luciListener {
	localIPs := map[string]bool{}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			localIPs[ip.String()] = true
		}
	}
	listeners := map[string]luciListener{}
	for _, scheme := range []string{"https", "http"} {
		bindings := commandOutput("uci", "-q", "get", "uhttpd.@uhttpd[0].listen_"+scheme)
		if listener := selectLuCIListener(bindings, localIPs); listener.port != 0 {
			listeners[scheme] = listener
		}
	}
	return listeners
}
