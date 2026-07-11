package main

import (
	"runtime/debug"
	"strings"

	"github.com/rs/zerolog/log"
)

func GetVersion() string {
	// Read the automatically injected build information
	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Warn().Msg("No build info available!")
	}

	var verStr []string = make([]string, 0)

	// Loop through the embedded settings to find Git info
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs":
			verStr = append(verStr, setting.Value)
		case "vcs.revision":
			verStr = append(verStr, setting.Value[:7])
		case "vcs.modified":
			if setting.Value == "true" {
				verStr = append(verStr, "dirty")
			}
		}
	}

	if len(verStr) == 0 {
		return "unknown"
	} else {
		return strings.Join(verStr, "-")
	}
}
