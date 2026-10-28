package main

import "regexp"

var labelReference = regexp.MustCompile(`\$\{([A-Za-z0-9_-]+)\}`)

func resolveLabel(label string, vars map[string]string) string {
	return labelReference.ReplaceAllStringFunc(label, func(token string) string {
		if value, ok := vars[token[2:len(token)-1]]; ok {
			return value
		}
		return token
	})
}
