//go:build windows

package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
)

//go:embed automation/text-uia.ps1
var textUIAScript string

func encodedPowerShell(s string) string {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(buf[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
func uiText(action, value string) (string, error) {
	return uiTextContext(context.Background(), action, value)
}
func uiTextContext(parent context.Context, action, value string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]string{"action": action, "value": value})
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-EncodedCommand", encodedPowerShell(textUIAScript))
	hideConsole(cmd)
	cmd.Stdin = bytes.NewReader(payload)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	var reply struct {
		OK    bool   `json:"ok"`
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if e := json.Unmarshal(bytes.TrimPrefix(bytes.TrimSpace(out.Bytes()), []byte{0xef, 0xbb, 0xbf}), &reply); e != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("UI Automation: перевищено час очікування")
		}
		return "", fmt.Errorf("UI Automation: не вдалося запустити адаптер (%v)", runErr)
	}
	if !reply.OK {
		return "", fmt.Errorf("UI Automation: %s", strings.TrimSpace(reply.Error))
	}
	return reply.Text, nil
}
