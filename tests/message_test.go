package main

import (
	"testing"

	"github.com/teejays/logdoc/config"
	"github.com/stretchr/testify/assert"
)

func TestNewLogMessageStructured(t *testing.T) {

	tests := []struct {
		name    string
		rawMsg  LogMessage
		want    LogMessageStructured
		wantErr bool
	}{
		{
			name:   "happy path",
			rawMsg: LogMessage{Message: `"10.0.0.5","-","apache",1549573963,"GET /api/user HTTP/1.0",200,1234`},
			want: LogMessageStructured{
				KV: map[string]string{
					"remotehost": "\"10.0.0.5\"",
					"rfc931":     "\"-\"",
					"authuser":   "\"apache\"",
					"date":       "1549573963",
					"request":    "\"GET /api/user HTTP/1.0\"",
					"status":     "200",
					"bytes":      "1234",
				},
			},
		},
		{
			name:    "error because of invalid dat",
			rawMsg:  LogMessage{Message: `"10.0.0.5",alabama, mexico, hohoho`},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := NewLogSourceSettingsFromConfig(config.ConfigLogSourceSettings{
				Format:          "csv",
				TimestampKey:    "date",
				TimestampFormat: "unix",
				Headers:         []string{"remotehost", "rfc931", "authuser", "date", "request", "status", "bytes"},
			})
			if err != nil {
				t.Errorf("could not set up log source settings: %s", err)
				return
			}

			got, err := NewLogMessageStructured(tt.rawMsg, settings)
			if tt.wantErr {
				assert.NotNil(t, err, "expected error to  be not nil")
				return
			}
			tt.want.LogMessage = tt.rawMsg
			assert.Equal(t, tt.rawMsg, got.LogMessage)
			assert.False(t, got.T.IsZero())
			assert.Equal(t, tt.want.KV, got.KV)
		})
	}
}
