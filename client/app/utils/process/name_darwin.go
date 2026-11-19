//go:build darwin

package process

func setProcessName(name string) {
	replaceArgvZero(name)
}
