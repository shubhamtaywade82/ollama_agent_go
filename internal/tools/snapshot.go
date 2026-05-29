package tools

import (
	"os"
)

// CaptureFileBefore reads the current content of a file before a mutating tool call.
// Returns nil (with no error) if the file does not yet exist (new-file scenario).
func CaptureFileBefore(root, relPath string) ([]byte, error) {
	if relPath == "" {
		return nil, nil
	}
	abs, err := resolvePath(root, relPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		return nil, nil // new file; compensation = delete
	}
	return data, err
}

// pathFromArgs extracts the "path" argument from a tool's args map.
// Returns an empty string if the key is absent or not a string.
func pathFromArgs(args map[string]any) string {
	if v, ok := args["path"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
