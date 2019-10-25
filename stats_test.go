package main

import (
	"testing"
	"time"

	"github.com/teejays/logdoc/config"
	"github.com/stretchr/testify/assert"
	"github.com/teejays/clog"
)

func VanillaStatsType(durationSeconds int64) (*StatsType, error) {
	req := config.ConfigStatsType{
		Name:            "Test Stats",
		DurationSeconds: durationSeconds,
		SourceSettings: []config.ConfigStatsTypeSourceSetting{
			{
				Name:                "test_source",
				Key:                 "request",
				ValueMutateFuncName: "HTTPStatusLineToSection",
				OtherKeys:           []string{"foo", "bar"},
			},
		},
	}

	c, err := NewStatsTypeFromConfig(req)
	if err != nil {
		return c, err
	}
	return c, nil
}

func TestStatsType_ConsumeLog(t *testing.T) {
	clog.LogLevel = 1
	now := time.Now()

	tests := []struct {
		name             string
		durationSeconds  int64
		priorLogMessages []LogMessageStructured
		priorAssertions  func(*testing.T, *StatsType)
		newLogMessage    LogMessageStructured
		postAssertions   func(*testing.T, *StatsType)
	}{
		{
			name:            "adding new log message should create a new window and trigger notification",
			durationSeconds: 2,
			priorLogMessages: []LogMessageStructured{
				{
					KV: map[string]string{"request": "GET /api/user HTTP/1.0", "foo": "foo1", "bar": "bar1"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "GET /api/user HTTP/1.0", "foo": "foo2", "bar": "bar2"},
					T:  now.Add(1 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "GET /api/user HTTP/1.0", "foo": "foo2", "bar": "bar2"},
					T:  now,
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
				{
					KV: map[string]string{"request": "GET /api/user HTTP/1.0", "foo": "foo2", "bar": "bar2"},
					T:  now.Add(1 * time.Second),
					LogMessage: LogMessage{
						SourceName: "test_source",
					},
				},
			},
			priorAssertions: func(t *testing.T, c *StatsType) {
				assert.Equal(t, 2, len(c.Windows))
				assert.Equal(t, 1, c.CurrentPointer)
				assert.Equal(t, now.Add(1*time.Second), c.LatestTimestamp)
				assert.Equal(t, 0, len(c.QueuedNotifications))
			},
			newLogMessage: LogMessageStructured{
				KV: map[string]string{"request": "GET /api/user HTTP/1.0"},
				T:  now.Add(2 * time.Second),
				LogMessage: LogMessage{
					SourceName: "test_source",
				},
			},
			postAssertions: func(t *testing.T, c *StatsType) {
				assert.Equal(t, 3, len(c.Windows))
				assert.Equal(t, 2, c.CurrentPointer)
				assert.Equal(t, now.Add(2*time.Second), c.LatestTimestamp)
				assert.Equal(t, []int{1}, c.QueuedNotifications)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Generate Vanilla StatsType
			c, err := VanillaStatsType(tt.durationSeconds)
			if err != nil {
				t.Errorf("could not generate vanilla StatsType: %s", err)
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

func TestStatsType_DetermineWindow(t *testing.T) {

	now := time.Now()
	tests := []struct {
		name       string
		duration   time.Duration
		priorTimes []time.Time
		t          time.Time
		want       int
	}{
		{
			name:     "test case",
			duration: 2 * time.Second,
			priorTimes: []time.Time{
				now,
				now,
				now.Add(-1 * time.Second),
				now,
				now.Add(time.Second),
				now.Add(time.Second),
				now.Add(2 * time.Second),
				now.Add(2 * time.Second),
				now.Add(2 * time.Second),
				now.Add(3 * time.Second),
			},
			t:    now.Add(3 * time.Second),
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewStatsTypeFromConfig(config.ConfigStatsType{})
			if err != nil {
				t.Errorf("error generating StatsType: %s", err)
				return
			}
			s.Duration = tt.duration
			s.PrepareForConsumption(now)

			for _, t := range tt.priorTimes {
				_, _ = s.determineWindow(t)
			}

			got, _ := s.determineWindow(tt.t)
			assert.Equal(t, tt.want, got)
		})
	}
}
