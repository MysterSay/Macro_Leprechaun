//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
	"os"
)

func platformAutostart(enabled bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		err = k.DeleteValue("MacroLeprechaun")
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue("MacroLeprechaun", `"`+exe+`"`)
}
