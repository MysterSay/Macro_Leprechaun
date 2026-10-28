package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Interface identities are ABI constants, checked against Microsoft's public
// IMMDevice definitions (microsoft/psi, ComInterop/Guids.cs).
func TestCoreAudioInterfaceIdentity(t *testing.T) {
	b, e := os.ReadFile("automation/audio.cs")
	if e != nil {
		t.Fatal(e)
	}
	for name, id := range map[string]string{"IEnum": "A95664D2-9614-4F35-A746-DE8DB63617E6", "ICollection": "0BD7A1BE-7A1A-44DB-8397-CC5392387B5E", "IDevice": "D666063F-1587-4E43-81F1-B948E807363F", "IEndpoint": "1BE09788-6894-4089-8586-9A2A6C265AC5", "IVolume": "5CDF2C82-841E-4546-9722-0CF74078229A"} {
		pattern := `Guid\("` + id + `"\)[^\n]*interface ` + name + `\b`
		if !regexp.MustCompile(pattern).Match(b) {
			t.Fatalf("%s must use Windows IID %s", name, id)
		}
	}
	if strings.Contains(string(b), "C0A359EA6D5F") {
		t.Fatal("invalid collection IID retained")
	}
}
