package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/teejays/clog"
	"github.com/teejays/logdoc/config"
)

func VanillaAlertType(durationSeconds int64, threshold int) (*AlertType, error) {
	req := config.ConfigAlertType{
		Name:            "Test Alert",
		DurationSeconds: durationSeconds,
		Threshold:       threshold,
		SourceSettings: []config.ConfigAlertTypeSourceSetting{
			{
				Name:                "test_source",
				Key:                 "",
				ValueMutateFuncName: "",
				Values:              nil,
			},
		},
	}

	c, err := NewAlertTypeFromConfig(req)
	if err != nil {
		return c, err
	}
	return c, nil
}

func TestAlertType_ConsumeLog(t *testing.T) {
	clog.LogLevel = 1
	now := time.Now()

	tests := []struct {
		name                 string
		durationSeconds      int64
		threshold            int
		priorLogMessages     []LogMessageStructured
		priorOldestNodeIndex int
		priorLatestNodeIndex int
		priorAssertions      func(*testing.T, *AlertType)
		newLogMessage        LogMessageStructured
		postAssertions       func(*testing.T, *AlertType)
	}{
		{
			name:            "adding the threshold log should trigger an alert",
			durationSeconds: 3,
			threshold:       5,
			priorLogMessages: []LogMessageStructured{
				{
					KV: map[string]string{"request": "/api/user"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/api/user"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now.Add(1 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
			},
			priorOldestNodeIndex: 0, // 1st element from pastLogMessages will be the oldest
			priorLatestNodeIndex: 2,
			priorAssertions: func(t *testing.T, c *AlertType) {
				assert.Equal(t, false, c.AlertOngoing)
				assert.Equal(t, 4, c.CurrentMovingCount)
			},
			newLogMessage: LogMessageStructured{
				KV: map[string]string{"request": "/report"},
				T:  now.Add(1 * time.Second),
				LogMessage: LogMessage{
					SourceName: "test_source",
				},
			},
			postAssertions: func(t *testing.T, c *AlertType) {
				assert.True(t, c.AlertOngoing)
				assert.Equal(t, 5, c.CurrentMovingCount)
			},
		},

		{
			name:            "adding a log which takes count to below threshold log should close the alert",
			durationSeconds: 3,
			threshold:       5,
			priorLogMessages: []LogMessageStructured{
				{
					KV: map[string]string{"request": "/api/user"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/api/user"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now.Add(1 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now.Add(1 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "/report"},
					T:  now.Add(2 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
			},
			priorOldestNodeIndex: 0, // 2nd element from pastLogMessages will be the oldest
			priorLatestNodeIndex: 5, // 2nd element from pastLogMessages will be the oldest
			priorAssertions: func(t *testing.T, c *AlertType) {
				assert.Equal(t, true, c.AlertOngoing)
				assert.Equal(t, 6, c.CurrentMovingCount)
			},
			newLogMessage: LogMessageStructured{
				KV: map[string]string{"request": "/report"},
				T:  now.Add(5 * time.Second),
				LogMessage: LogMessage{
					SourceName: "test_source",
				},
			},
			postAssertions: func(t *testing.T, c *AlertType) {
				assert.False(t, c.AlertOngoing)
				assert.Equal(t, 2, c.CurrentMovingCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Generate Vanilla AlertType
			c, err := VanillaAlertType(tt.durationSeconds, tt.threshold)
			if err != nil {
				t.Errorf("could not generate vanilla AlertType: %s", err)
				return
			}
			_ = c.PrepareForConsumption(now)

			// Run previous logs on it
			for _, msg := range tt.priorLogMessages {
				err = c.ConsumeLog(msg)
				if err != nil {
					t.Errorf("could not consume a previous log %+v: %s", msg, err)
					return
				}
			}

			// Prior Assertions
			assert.Equal(t, tt.priorLogMessages[tt.priorLatestNodeIndex].LogMessage, c.LatestLogNode.LogMessage)
			assert.Equal(t, tt.priorLogMessages[tt.priorOldestNodeIndex].LogMessage, c.OldestLogNode.LogMessage)

			if tt.priorAssertions != nil {
				tt.priorAssertions(t, c)
				if t.Failed() {
					return
				}
			}

			// Run Main Func.
			err = c.ConsumeLog(tt.newLogMessage)
			if err != nil {
				t.Errorf("could not run main function: %s", err)
				return
			}
			if tt.postAssertions != nil {
				tt.postAssertions(t, c)
				if t.Failed() {
					return
				}
			}

		})
	}
}
