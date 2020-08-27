package main

import (
	"net"
)

// ParseIPPort returns a valid string that can be passed to net.Dial, containing both the IP
// address and the port number.
func ParseIPPort(input string) (string, error) {
	if ip := net.ParseIP(input); ip != nil {
		// A "pure" IP was passed, with no port number (or name)
		return net.JoinHostPort(ip.String(), "53"), nil
	}
	// Input has both address and port
	host, port, err := net.SplitHostPort(input)
	if err != nil {
		return input, err
	}
	return net.JoinHostPort(host, port), nil
}
