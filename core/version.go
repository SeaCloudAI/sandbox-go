package core

import "strings"

var SDKVersion = "dev"

func UserAgent(name string) string {
	version := strings.TrimSpace(SDKVersion)
	if version == "" {
		version = "dev"
	}
	return name + "/" + version
}
