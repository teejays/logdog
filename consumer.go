package main

import (
	"fmt"
	"sync"
	"time"
)

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// LogConsumer is anything that wants to consume a structured LogMessage.
type LogConsumer interface {
	GetName() string
	GetChannel() chan LogMessageStructured
	NumConsumed() int
	GetSourceSettings(srcName string) (LogConsumerSourceSettings, error)
	GetAllSourceSettings() map[string]LogConsumerSourceSettings

	PrepareForConsumption(currentTime time.Time) error
	ConsumeLog(lg LogMessageStructured) error
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  -  B A S E  C O N S U M E R
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */
type baseLogConsumer struct {
	Name           string
	ConsumedCount  int
	SourceSettings map[string]LogConsumerSourceSettings
	InQueue        chan LogMessageStructured
	Lock           *sync.RWMutex
}

func (c *baseLogConsumer) GetName() string {
	return c.Name
}

func (c *baseLogConsumer) GetChannel() chan LogMessageStructured {
	return c.InQueue
}
func (c *baseLogConsumer) NumConsumed() int {
	return c.ConsumedCount
}

func (c *baseLogConsumer) GetSourceSettings(srcName string) (LogConsumerSourceSettings, error) {
	settings, exists := c.SourceSettings[srcName]
	if !exists {
		return settings, fmt.Errorf("source settings not found")
	}
	return settings, nil
}

func (c *baseLogConsumer) GetAllSourceSettings() map[string]LogConsumerSourceSettings {
	return c.SourceSettings
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  S O U R C E  S E T T I N G S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type LogConsumerSourceSettings interface {
	GetKey() string
	GetCleanedValue(lg LogMessageStructured) (string, error)
	GetCleanedKV(lg LogMessageStructured) (map[string]string, error)
	IsMatch(lg LogMessageStructured) (bool, error)
}
