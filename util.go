package main

import (
	"fmt"
	"sort"
	"strings"
)

func StripTrailingNewlineCharacter(text string) string {
	if len(text) < 1 {
		return text
	}
	if text[len(text)-1] == '\n' {
		return text[:len(text)-1] // or strings.ReplaceAll(text, "\n", "")
	}
	return text
}

// HTTPStatusLineToSection converts a `GET /api/user HTTP/1.0` to `/api`.
func HTTPStatusLineToSection(text string) (string, error) {
	// Split the HTTP Request header by whitespace
	parts := strings.Split(text, " ")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid number of parts: expected %d, got %d", 3, len(parts))
	}
	// Take the middle part, which is the endpoint and split it by /
	endpoint := parts[1]
	endpointParts := strings.Split(endpoint, "/")
	if len(endpointParts) < 1 {
		return "", fmt.Errorf("no endpoint parts detected, is the endpoint empty?")
	}
	// Since the endpoints begin with a `/`, we should expect the first split part to be an empty string
	if endpointParts[0] != "" {
		return "", fmt.Errorf("expected endpoint to begin with a `/`: %s", endpoint)
	}
	if len(endpointParts) < 2 {
		return "", fmt.Errorf("not enough info in the endpoint string to extract website section: %s", endpoint)
	}
	return "/" + endpointParts[1], nil
}

func filterMapCopy(kv map[string]string, keyMask []string) map[string]string {
	var m = make(map[string]string)
	for _, k := range keyMask {
		m[k] = kv[k]
	}
	return m
}

// removeQuotes takes a string and strips `"` if they wrap the string. e.g. ``"hello"`` -> `hello`, and `"hello` -> `"hello`.
func removeQuotes(str string) string {
	if len(str) > 1 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return str
}

func sortMapKeysByValue(kv map[string]int) []string {
	// Reverse the map
	var vk = make(map[int][]string)
	for k, v := range kv {
		vk[v] = append(vk[v], k)
	}

	// Get keys
	var vs []int
	for v := range vk {
		vs = append(vs, v)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(vs)))

	var sortedKs []string
	for _, v := range vs {
		sortedKs = append(sortedKs, vk[v]...)
	}
	return sortedKs

}
