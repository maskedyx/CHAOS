//go:build linux || darwin

package process

import (
	"os"
	"unsafe"
)

// replaceArgvZero overwrites argv[0] in place, which is what /proc/<pid>/cmdline
// and process listings report. The name is truncated to the original argv[0]
// length so the neighbouring argv/env memory is left untouched.
func replaceArgvZero(name string) {
	if len(os.Args) == 0 || len(os.Args[0]) == 0 {
		return
	}

	argv0 := unsafe.Slice(unsafe.StringData(os.Args[0]), len(os.Args[0]))
	for i := range argv0 {
		if i < len(name) {
			argv0[i] = name[i]
			continue
		}
		argv0[i] = 0
	}
}
