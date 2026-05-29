package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

func resolvePath(root, p string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	var target string
	if filepath.IsAbs(p) {
		target = filepath.Clean(p)
	} else {
		target = filepath.Clean(filepath.Join(absRoot, p))
	}

	if target != absRoot && !strings.HasPrefix(target, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes sandbox root %q", p, absRoot)
	}
	return target, nil
}

func relPath(root, target string) string {
	if rel, err := filepath.Rel(root, target); err == nil {
		return rel
	}
	return target
}

func argString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string, got %T", key, v)
	}
	return s, nil
}

func argStringOpt(args map[string]any, key, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func argBoolOpt(args map[string]any, key string, def bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}
