package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// parseSetFlags turns repeated `--set key=value` pairs into a nested map.
//
//	--set name=Acme                     {"name": "Acme"}
//	--set security.dpop_require_nonce=true
//	                                    {"security": {"dpop_require_nonce": true}}
//	--set tags='["a","b"]'              JSON values are parsed
//	--set note=null                     null clears a key
//
// Values are coerced: true/false → bool, null → nil, integers and decimals →
// numbers, `{…}`/`[…]` → JSON, anything else → string. Wrap a value in
// quotes (str:"123") to force a string.
func parseSetFlags(pairs []string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	for _, pair := range pairs {
		key, raw, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("--set expects key=value, got %q", pair)
		}
		setPath(out, strings.Split(key, "."), coerceValue(raw))
	}
	return out, nil
}

func setPath(m map[string]interface{}, path []string, value interface{}) {
	if len(path) == 1 {
		m[path[0]] = value
		return
	}
	child, ok := m[path[0]].(map[string]interface{})
	if !ok {
		child = map[string]interface{}{}
		m[path[0]] = child
	}
	setPath(child, path[1:], value)
}

func coerceValue(raw string) interface{} {
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, "str:") {
		return strings.TrimPrefix(v, "str:")
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
	}
	return v
}

// mergeInto copies src into dst (dst wins nothing; src overrides).
func mergeInto(dst, src map[string]interface{}) {
	for k, v := range src {
		if sv, ok := v.(map[string]interface{}); ok {
			if dv, ok := dst[k].(map[string]interface{}); ok {
				mergeInto(dv, sv)
				continue
			}
		}
		dst[k] = v
	}
}
