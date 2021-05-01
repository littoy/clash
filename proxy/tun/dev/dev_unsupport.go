// +build !linux,!android,!darwin,!windows

package dev

import (
	"errors"
	"runtime"

	"net/url"
)

func OpenTunDevice(_ url.URL) (TunDevice, error) {
	return nil, errors.New("Unsupported platform " + runtime.GOOS + "/" + runtime.GOARCH)
}

// GetAutoDetectInterface get ethernet interface
func GetAutoDetectInterface() (string, error) {
	return "", nil
}
