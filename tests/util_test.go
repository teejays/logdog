package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterMapCopy(t *testing.T) {

	tests := []struct {
		name         string
		in_kv        map[string]string
		in_fieldMask []string
		expected     map[string]string
	}{
		{
			name: "non-edge case",
			in_kv: map[string]string{
				"a": "_a",
				"b": "_b",
				"c": "_c",
				"d": "_d",
				"e": "_e",
			},
			in_fieldMask: []string{"b", "c"},
			expected: map[string]string{
				"b": "_b",
				"c": "_c",
			},
		},
		{
			name: "empty keys work",
			in_kv: map[string]string{
				"a": "_a",
				"b": "_b",
				"c": "_c",
				"d": "_d",
				"e": "_e",
				"":  "_",
			},
			in_fieldMask: []string{"b", "c", ""},
			expected: map[string]string{
				"b": "_b",
				"c": "_c",
				"":  "_",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterMapCopy(tt.in_kv, tt.in_fieldMask)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHTTPStatusLineToSection(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name     string
		in_text  string
		expected string
		wantErr  bool
	}{
		{
			name:     "normal case",
			in_text:  "GET /api/user HTTP/1.0",
			expected: "/api",
		},
		{
			name:     "just the section",
			in_text:  "GET /api HTTP/1.0",
			expected: "/api",
		},
		{
			name:     "long one",
			in_text:  "GET /report/api/xyz?123/ HTTP/1.0",
			expected: "/report",
		},
		{
			name:     "trailing slash",
			in_text:  "GET /api/ HTTP/1.0",
			expected: "/api",
		},
		{
			name:    "no starting slash",
			in_text: "GET api/ HTTP/1.0",
			wantErr: true,
		},
		{
			name:    "wrong format for line",
			in_text: "api/ GET HTTP/1.0 GET",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPStatusLineToSection(tt.in_text)
			if tt.wantErr {
				assert.NotNil(t, err, "expected error")
				return
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestStripTrailingNewlineCharacter(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty string", "", ""},
		{"non new-line characted", "abc\r", "abc\r"},
		{"non new-line characted", "\n", ""},
		{"new-line characted", "abc\n", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripTrailingNewlineCharacter(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}
