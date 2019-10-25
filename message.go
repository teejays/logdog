package main

import (
	"fmt"
	"time"

	"github.com/teejays/clog"
)

type LogMessage struct {
	SourceName     string
	Message        string
	Id             int64
	IsCancelSignal bool
}

type LogMessageStructured struct {
	LogMessage
	KV map[string]string
	T  time.Time
}

func (lg LogMessageStructured) GetValue(k string) string {
	return lg.KV[k]
}

func NewLogMessageStructured(rawMsg LogMessage, settings LogSourceSettings) (LogMessageStructured, error) {

	var msg LogMessageStructured

	// Make a KV map so the log is structured
	kv, err := settings.Format.GetKeyValueMap(rawMsg.Message, settings.Headers)
	if err != nil {
		return msg, fmt.Errorf("creating a key-value map for source %s: %w", rawMsg.SourceName, err)
	}

	// Get the timestamp
	timeStr := kv[settings.TimestampKey]
	clog.Debugf("[%s] [%d] Time string value fetched using key '%s': %s", rawMsg.SourceName, rawMsg.Id, settings.TimestampKey, timeStr)

	var t time.Time
	t, err = settings.TimestampFormat.Parse(timeStr)
	if err != nil {
		return msg, fmt.Errorf("parsing %s to time: %w", timeStr, err)
	}
	clog.Debugf("[%s] [%d] Timestamp fetched: %s", rawMsg.SourceName, rawMsg.Id, t)

	// Create a structure log message

	msg.KV = kv
	msg.T = t
	msg.LogMessage = rawMsg

	return msg, nil

}
