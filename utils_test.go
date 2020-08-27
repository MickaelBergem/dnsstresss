package main

import "testing"

func TestParseIPPort(t *testing.T) {
	tables := []struct {
		input    string
		expected string
	}{
		// Explicit IP and port
		{"127.0.0.1:53", "127.0.0.1:53"},
		{"1.1.1.1:5353", "1.1.1.1:5353"},
		// The function should add the implicit port
		{"127.0.0.1", "127.0.0.1:53"},
		// Should work with IPv6 addresses (with no brackets)
		// (see https://github.com/MickaelBergem/dnsstresss/issues/3#issuecomment-160758393)
		{"2001:4b98:dc2:45:216:3eff:fe4b:8c5b", "[2001:4b98:dc2:45:216:3eff:fe4b:8c5b]:53"},
		{"[2001:4b98:dc2:45:216:3eff:fe4b:8c5b]:53", "[2001:4b98:dc2:45:216:3eff:fe4b:8c5b]:53"},
	}

	for _, table := range tables {
		result, _ := ParseIPPort(table.input)
		if result != table.expected {
			t.Errorf("Invalid parsing of input %s: got %s but expected %s", table.input, result, table.expected)
		}
	}

	// Invalid input
	_, err := ParseIPPort("2001:4b98:dc2:45:216:3eff:fe4b:8c5b:53")
	if err == nil {
		t.Error("Invalid inputs should return a non-nil error")
	}
}
