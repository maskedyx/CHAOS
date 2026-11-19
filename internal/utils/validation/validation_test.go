package validation

import "testing"

func TestIsValidHostname(t *testing.T) {
	valid := []string{"localhost", "agent", "example.com", "sub.example.com", "host-1.example.com"}
	for _, hostname := range valid {
		if !IsValidHostname(hostname) {
			t.Errorf("IsValidHostname(%q) = false, want true", hostname)
		}
	}

	invalid := []string{"", "-bad.com", "bad-.com", "bad..com", "bad_host.com", "bad host"}
	for _, hostname := range invalid {
		if IsValidHostname(hostname) {
			t.Errorf("IsValidHostname(%q) = true, want false", hostname)
		}
	}
}

func TestIsValidMacAddress(t *testing.T) {
	valid := []string{
		"AA:BB:CC:DD:EE:FF",
		"aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF",
	}
	for _, address := range valid {
		if !IsValidMacAddress(address) {
			t.Errorf("IsValidMacAddress(%q) = false, want true", address)
		}
	}

	invalid := []string{"", "AA:BB:CC:DD:EE", "not-a-mac", "AA:BB:CC:DD:EE:GG"}
	for _, address := range invalid {
		if IsValidMacAddress(address) {
			t.Errorf("IsValidMacAddress(%q) = true, want false", address)
		}
	}
}
