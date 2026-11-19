package system

import "testing"

func TestParseOSTarget(t *testing.T) {
	tests := []struct {
		value  string
		target OSType
		found  bool
	}{
		{"1", Windows, true},
		{"2", Linux, true},
		{"3", Darwin, true},
		{"windows", Windows, true},
		{"linux", Linux, true},
		{"darwin", Darwin, true},
		{"macos", Darwin, true},
		{"Mac OS", Darwin, true},
		{"mac", Darwin, true},
		{"9", Unknown, false},
		{"", Unknown, false},
		{"plan9", Unknown, false},
	}

	for _, test := range tests {
		target, found := ParseOSTarget(test.value)
		if found != test.found || target != test.target {
			t.Errorf("ParseOSTarget(%q) = (%v, %v), want (%v, %v)", test.value, target, found, test.target, test.found)
		}
	}
}

func TestOSTargetMaps(t *testing.T) {
	if OSTargetMap[Darwin] != "Mac OS" {
		t.Errorf("OSTargetMap[Darwin] = %q, want %q", OSTargetMap[Darwin], "Mac OS")
	}
	if OSTargetGoOSMap[Darwin] != "darwin" {
		t.Errorf("OSTargetGoOSMap[Darwin] = %q, want %q", OSTargetGoOSMap[Darwin], "darwin")
	}
	if OSTargetIntMap[3] != Darwin {
		t.Errorf("OSTargetIntMap[3] = %v, want %v", OSTargetIntMap[3], Darwin)
	}
}

func TestIsSupportedArch(t *testing.T) {
	tests := []struct {
		target OSType
		arch   string
		want   bool
	}{
		{Darwin, "amd64", true},
		{Darwin, "arm64", true},
		{Linux, "arm64", true},
		{Windows, "amd64", true},
		{Windows, "arm64", false},
		{Unknown, "amd64", false},
	}

	for _, test := range tests {
		if got := IsSupportedArch(test.target, test.arch); got != test.want {
			t.Errorf("IsSupportedArch(%v, %q) = %v, want %v", test.target, test.arch, got, test.want)
		}
	}
}

func TestDefaultArch(t *testing.T) {
	if got := DefaultArch(Darwin); got != "amd64" {
		t.Errorf("DefaultArch(Darwin) = %q, want %q", got, "amd64")
	}
	if got := DefaultArch(Unknown); got != "" {
		t.Errorf("DefaultArch(Unknown) = %q, want an empty string", got)
	}
}
