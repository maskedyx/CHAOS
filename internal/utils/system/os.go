package system

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type OSType int

const (
	Unknown OSType = iota
	Windows
	Linux
	Darwin
)

var OSTargetMap = map[OSType]string{
	Windows: "Windows",
	Linux:   "Linux",
	Darwin:  "Mac OS",
}

var OSTargetIntMap = map[int]OSType{
	1: Windows,
	2: Linux,
	3: Darwin,
}

var OSTargetGoOSMap = map[OSType]string{
	Windows: "windows",
	Linux:   "linux",
	Darwin:  "darwin",
}

var OSTargetArchMap = map[OSType][]string{
	Windows: {"amd64"},
	Linux:   {"amd64", "arm64"},
	Darwin:  {"amd64", "arm64"},
}

// ParseOSTarget accepts a numeric or textual target and returns its OS type.
func ParseOSTarget(value string) (OSType, bool) {
	value = strings.TrimSpace(strings.ToLower(value))

	for target, name := range OSTargetGoOSMap {
		if value == name {
			return target, true
		}
	}
	switch value {
	case "mac", "macos", "mac os", "osx":
		return Darwin, true
	}

	var index int
	if _, err := fmt.Sscanf(value, "%d", &index); err == nil {
		target, found := OSTargetIntMap[index]
		return target, found
	}
	return Unknown, false
}

func DefaultArch(target OSType) string {
	archs, found := OSTargetArchMap[target]
	if !found || len(archs) == 0 {
		return ""
	}
	return archs[0]
}

func IsSupportedArch(target OSType, arch string) bool {
	archs, found := OSTargetArchMap[target]
	if !found {
		return false
	}
	for _, supported := range archs {
		if arch == supported {
			return true
		}
	}
	return false
}

// DetectOS return an int which represent an OS type
func DetectOS() OSType {
	switch runtime.GOOS {
	case `windows`:
		return Windows
	case `linux`:
		return Linux
	case `darwin`:
		return Darwin
	default:
		return Unknown
	}
}

// ClearScreen clean the terminal
func ClearScreen() error {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
