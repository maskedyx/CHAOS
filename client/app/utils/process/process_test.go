package process

import "testing"

func TestParseMaskName(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"", false},
		{"invalid", false},
	}

	for _, test := range tests {
		if got := ParseMaskName(test.value); got != test.want {
			t.Errorf("ParseMaskName(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestParseRunHidden(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"invalid", false},
	}

	for _, test := range tests {
		if got := ParseRunHidden(test.value); got != test.want {
			t.Errorf("ParseRunHidden(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}
