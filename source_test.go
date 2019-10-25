package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogSourceFormat_CSV_GetPartsFromText(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		stripQuotes bool
		want        []string
	}{
		{
			name:        "simple case",
			text:        `foo,bar,baz,yaz`,
			stripQuotes: false,
			want:        []string{"foo", "bar", "baz", "yaz"},
		},
		{
			name:        "leave half quotes on",
			text:        `foo,bar,"baz",yaz`,
			stripQuotes: false,
			want:        []string{"foo", "bar", "\"baz\"", "yaz"},
		},
		{
			name:        "strip quotes",
			text:        `foo,bar,"baz",yaz`,
			stripQuotes: true,
			want:        []string{"foo", "bar", "baz", "yaz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s := LogSourceFormat_CSV{}
			got := s.GetPartsFromText(tt.text, tt.stripQuotes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogSourceFormat_CSV_GetKeyValueMap(t *testing.T) {

	tests := []struct {
		name    string
		text    string
		headers []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "happy case",
			text:    `foo,bar,baz,yaz`,
			headers: []string{"hfoo", "hbar", "hbaz", "hyaz"},
			want:    map[string]string{"hfoo": "foo", "hbar": "bar", "hbaz": "baz", "hyaz": "yaz"},
			wantErr: false,
		},
		{
			name:    "error num headers differ then num elems",
			text:    `foo,bar,baz,yaz`,
			headers: []string{"hfoo", "hbar", "hbaz"},
			wantErr: true,
		},
		{
			name:    "error num headers differ then num elems",
			text:    `foo,bar,baz`,
			headers: []string{"hfoo", "hbar", "hbaz", "hyaz"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := LogSourceFormat_CSV{}
			got, err := s.GetKeyValueMap(tt.text, tt.headers)
			if tt.wantErr {
				assert.NotNil(t, err, "expected err")
				return
			}
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestTimestampFormat_Unix_Parse(t *testing.T) {

	tests := []struct {
		name    string
		str     string
		wantErr bool
	}{
		{
			name:    "happy path",
			str:     "1549573963",
			wantErr: false,
		},
		{
			name:    "error is str is not unix timestamp",
			str:     "ljh123jh",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TimestampFormat_Unix{}
			got, err := s.Parse(tt.str)
			if tt.wantErr {
				assert.NotNil(t, err, "expected error to  be not nil")
				return
			}
			assert.False(t, got.IsZero())

		})
	}
}
