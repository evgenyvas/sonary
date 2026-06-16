// Package utils
package utils

import (
	"strings"
	"unicode"
)

// Ptr cannot return new(v) because of zero value
func Ptr[T any](v T) *T {
	return &v
}

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

// ChunkMap splits a map into a slice of smaller maps of a maximum size
func ChunkMap[K comparable, V any](originalMap map[K]V, chunkSize int) []map[K]V {
	if chunkSize <= 0 {
		return []map[K]V{originalMap}
	}

	var chunks []map[K]V
	currentChunk := make(map[K]V)

	for key, value := range originalMap {
		currentChunk[key] = value

		// Once the current chunk reaches the max size, save it and start a new one
		if len(currentChunk) == chunkSize {
			chunks = append(chunks, currentChunk)
			currentChunk = make(map[K]V)
		}
	}

	// Add the final chunk if it contains any remaining items
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}
