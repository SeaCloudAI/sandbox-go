package core

import "strings"

var SDKVersion = "0.2.0"

func UserAgent(name string) string {
	version := strings.TrimSpace(SDKVersion)
	if version == "" {
		version = "dev"
	}
	return name + "/" + version
}
