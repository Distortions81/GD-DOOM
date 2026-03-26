package doomruntime

import (
	"os"
	"strings"
)

var runtimeDebugEnvCache = map[string]string{}

func loadRuntimeDebugEnvFromOS() {
	cache := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.HasPrefix(k, "GD_DEBUG_") || strings.HasPrefix(k, "GD_TRACE_") {
			cache[k] = v
		}
	}
	runtimeDebugEnvCache = cache
}

func runtimeDebugEnv(key string) string {
	return runtimeDebugEnvCache[key]
}
