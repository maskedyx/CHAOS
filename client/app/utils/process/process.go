package process

import "strconv"

// ParseRunHidden converts the build-time RunHidden value into a boolean.
func ParseRunHidden(value string) bool {
	hidden, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return hidden
}

// ParseMaskName converts the build-time MaskName value into a boolean.
func ParseMaskName(value string) bool {
	masked, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return masked
}
