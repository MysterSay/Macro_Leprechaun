package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
)

func moduleDirectory(m Module) string {
	if m.directory != "" {
		return m.directory
	}
	if m.Entry != "" {
		return filepath.Dir(m.Entry)
	}
	return filepath.Dir(m.Executable)
}
func psLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
func powershellAddonWrapper(m Module) string {
	args := []string{}
	for _, arg := range m.Args {
		args = append(args, psLiteral(arg))
	}
	// Read UTF-8 explicitly: Windows PowerShell 5.1 otherwise treats BOM-less
	// files as the ANSI code page. JSON on stdin stays separate from script code.
	return `[Console]::InputEncoding=New-Object Text.UTF8Encoding($false);[Console]::OutputEncoding=New-Object Text.UTF8Encoding($false);$OutputEncoding=[Console]::OutputEncoding;$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue';$VerbosePreference='SilentlyContinue';try { & ([scriptblock]::Create([IO.File]::ReadAllText(` + psLiteral(m.Entry) + `,[Text.Encoding]::UTF8))) ` + strings.Join(args, " ") + ` } catch { [Console]::Error.WriteLine($_.Exception.Message);@{ok=$false;error=$_.Exception.Message}|ConvertTo-Json -Compress -Depth 20;exit 1 }`
}
func encodedAddonCommand(source string) string {
	units := utf16.Encode([]rune(source))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(data)
}
func moduleCommand(ctx context.Context, m Module) (*exec.Cmd, error) {
	if m.Entry == "" {
		if m.Executable == "" {
			return nil, fmt.Errorf("addon entry missing")
		}
		cmd := exec.CommandContext(ctx, m.Executable, m.Args...)
		cmd.Dir = moduleDirectory(m)
		return cmd, nil
	}
	info, err := os.Stat(m.Entry)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("addon script missing: %s", m.Entry)
	}
	var program string
	var args []string
	switch m.Runtime {
	case "powershell":
		program = "powershell.exe"
		if runtime.GOOS != "windows" {
			program = "pwsh"
		}
		args = []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
		if runtime.GOOS == "windows" {
			args = append(args, "-STA")
		}
		args = append(args, "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedAddonCommand(powershellAddonWrapper(m)))
	case "python":
		candidates := []string{"python3", "python"}
		if runtime.GOOS == "windows" {
			candidates = []string{"python.exe", "py.exe"}
		}
		for _, candidate := range candidates {
			if found, e := exec.LookPath(candidate); e == nil {
				program = found
				if strings.EqualFold(filepath.Base(candidate), "py.exe") {
					args = append(args, "-3")
				}
				break
			}
		}
		args = append(args, "-u", m.Entry)
		args = append(args, m.Args...)
	case "javascript":
		program = "node"
		args = append([]string{m.Entry}, m.Args...)
	default:
		return nil, fmt.Errorf("unsupported addon runtime %q", m.Runtime)
	}
	if program == "" {
		return nil, fmt.Errorf("інтерпретатор %s не знайдено", m.Runtime)
	}
	resolved, err := exec.LookPath(program)
	if err != nil {
		return nil, fmt.Errorf("інтерпретатор %s не знайдено: %w", m.Runtime, err)
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	cmd.Dir = moduleDirectory(m)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1", "MACROLEPRECHAUN_ADDON_DIR="+moduleDirectory(m))
	return cmd, nil
}
