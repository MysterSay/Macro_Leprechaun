//go:build !windows

package main

import (
	"context"
	"fmt"
)

func windowsInventory() (any, error) {
	return map[string]any{"terminals": []any{}, "apps": []any{}, "devices": []any{}, "platform": "unsupported"}, nil
}
func windowsPick(kind string) (string, error) { return "", fmt.Errorf("потрібна Windows") }
func windowsCursor() (any, error)             { return nil, fmt.Errorf("потрібна Windows") }
func executeWindowsAction(kind string, q WinAction) error {
	return fmt.Errorf("пазли Win потребують Windows")
}

func executeWindowsActionContext(ctx context.Context, kind string, q WinAction) error {
	return executeWindowsAction(kind, q)
}
