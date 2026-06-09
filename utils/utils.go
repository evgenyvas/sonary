// Package utils
package utils

import (
	"strings"
	"unicode"
)

func CleanString(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, str)
}

func KeepASCII(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r <= 127 {
			result = append(result, r)
		}
	}
	return string(result)
}
