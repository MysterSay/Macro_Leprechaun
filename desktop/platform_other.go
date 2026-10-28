//go:build !windows

package main

import (
	"context"
	"fmt"
	"os/exec"
)

func hideConsole(c *exec.Cmd) {}
func shellCommand(c context.Context, s string) *exec.Cmd {
	return exec.CommandContext(c, "sh", "-c", s)
}
func typeText(s string) error       { return fmt.Errorf("text automation requires Windows") }
func sendKeys(s string) error       { return fmt.Errorf("keyboard automation requires Windows") }
func openSettings(u string)         { _ = exec.Command("xdg-open", u).Start() }
func platformRefresh()              {}
func platformLoop(a *App, u string) { select {} }

func platformNavigationRefresh()         {}
func readSelectedText() (string, error)  { return "", fmt.Errorf("потрібна Windows") }
func readClipboardText() (string, error) { return "", fmt.Errorf("потрібна Windows") }

func platformNetworkName() string { return "" }

func configureCommandCancellation(cmd *exec.Cmd) {}

func typeTextContext(ctx context.Context, s string) error         { return typeText(s) }
func readSelectedTextContext(ctx context.Context) (string, error) { return readSelectedText() }

func platformAutostart(enabled bool) error { return nil }

func platformKeyboardLayout() string { return "unavailable" }

func runSettingsWindow(target, directory string, parent int) error {
	return fmt.Errorf("WebView2 is available on Windows")
}

func runFloatingWindow(target, directory string, parent int) error {
	return fmt.Errorf("Floating WebView2 requires Windows")
}
func reportFloatingFailure(err error) {}
