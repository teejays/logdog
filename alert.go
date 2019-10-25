package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/teejays/clog"
	"github.com/teejays/logdoc/config"
)

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  -  A L E R T
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type AlertType struct {
	Duration  time.Duration
	Threshold int
	baseLogConsumer

	CurrentMovingCount int
	LatestLogNode      *AlertLogNode
	OldestLogNode      *AlertLogNode
	AlertOngoing       bool
	Alerts             []Alert
}

func NewAlertTypeFromConfig(req config.ConfigAlertType) (*AlertType, error) {
	var c = AlertType{
		Duration:  time.Duration(req.DurationSeconds * int64(time.Second)),
		Threshold: req.Threshold,
	}
	c.baseLogConsumer.Name = req.Name
	c.baseLogConsumer.Lock = &sync.RWMutex{}
	c.baseLogConsumer.InQueue = make(chan LogMessageStructured, 8)

	var sourceSettingsMap = make(map[string]LogConsumerSourceSettings)
	for _, cfgSettings := range req.SourceSettings {

		if _, exists := sourceSettingsMap[cfgSettings.Name]; exists {
			return nil, fmt.Errorf("multiple stats source settings for log source found")
		}

		settings, err := NewAlertTypeSourceSettings(cfgSettings.Key, cfgSettings.ValueMutateFuncName, cfgSettings.Values)
		if err != nil {
			return nil, err
		}

		sourceSettingsMap[cfgSettings.Name] = settings
	}

	c.baseLogConsumer.SourceSettings = sourceSettingsMap

	return &c, nil
}

func (c *AlertType) PrepareForConsumption(currentTime time.Time) error {
	// Nothing needs to be done...
	return nil
}

func (c *AlertType) ConsumeLog(msg LogMessageStructured) error {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	// Get the config of how to handle a log message from this source for this particular Stats
	settings, err := c.GetSourceSettings(msg.SourceName)
	if err != nil {
		return err
	}

	shouldInclude, err := settings.IsMatch(msg)
	if err != nil {
		return err
	}

	// If we're including this log message in our alertType, see where to include it
	if !shouldInclude {
		return nil
	}

	clog.Debugf("[%s] [%d] [%s] Including log in the alert...", msg.SourceName, msg.Id, c.Name)

	// Inject the  new log into the chain
	err = c.addToChain(msg)
	if err != nil {
		return err
	}
	clog.Debugf("[%s] [%d] [%s] LogNode inserted.\nLatestNode: %+v\nOldestNode: %+v", msg.SourceName, msg.Id, c.Name, c.LatestLogNode, c.OldestLogNode)

	// Tree shake the cache to: start loop from the very bottom and reach a point at which we can cut off the tail
	numRemoved, err := c.dropOldLogMessages()
	if err != nil {
		return err
	}
	clog.Debugf("[%s] [%d] [%s] Tree shaking complete: %d nodes removed", msg.SourceName, msg.Id, c.Name, numRemoved)

	// Update the count?
	c.CurrentMovingCount = c.CurrentMovingCount + 1 - numRemoved // + 1 for the new node which was added
	clog.Debugf("[%s] [%d] [%s] Chain Count: %d", msg.SourceName, msg.Id, c.Name, c.CurrentMovingCount)

	// Does the count signal a state of alert?
	var alertMode bool
	if c.CurrentMovingCount >= c.Threshold {
		alertMode = true
	}

	// If we should alert, and there is an ongoing alert already... don't do much
	latestTimestamp := c.LatestLogNode.T
	// If we should alert, but no ongoing alert
	if alertMode {
		c.triggerAlert(latestTimestamp)
	}

	// If we should not alert, but an ongoing alert, then close it down
	if !alertMode {
		c.closeAlert(latestTimestamp)
	}

	return nil

}

func (c *AlertType) triggerAlert(start time.Time) {
	if c.AlertOngoing {
		return
	}

	alert := Alert{
		Start: start,
	}
	c.Alerts = append(c.Alerts, alert)
	c.AlertOngoing = true

	// Do something!
	clog.Noticef("%s generated an alert - hits = %d, triggered at %s", c.Name, c.CurrentMovingCount, start)

}
func (c *AlertType) closeAlert(end time.Time) {
	if !c.AlertOngoing {
		return
	}
	c.Alerts[len(c.Alerts)-1].End = end
	c.AlertOngoing = false

	clog.Noticef("High traffic alert recovered at %s", end)

}

func (s *AlertType) addToChain(msg LogMessageStructured) error {

	// Inject the  new log into the chain
	var currentNode = s.LatestLogNode

	// Loop and find the log node before which we should place this log.
	for {
		if currentNode != nil && msg.T.Before(currentNode.T) {
			currentNode = currentNode.Previous
			continue
		}

		var newLogNode AlertLogNode
		newLogNode.LogMessageStructured = msg
		newLogNode.Previous = currentNode

		if currentNode != nil {
			newLogNode.Next = currentNode.Next
			currentNode.Next = &newLogNode
		}

		// if we're adding as last node
		if newLogNode.Next == nil {
			s.LatestLogNode = &newLogNode
		}

		// if we're placing it at first node
		if newLogNode.Previous == nil {
			s.OldestLogNode = &newLogNode
		}

		break
	}
	return nil
}

func (s *AlertType) dropOldLogMessages() (int, error) {

	// Tree shake the cache to: start loop from the very bottom and reach a point at which we can cut off the tail
	latestTimestamp := s.LatestLogNode.T
	currentNode := s.OldestLogNode
	var numNodesRemoved int

	for {
		if latestTimestamp.Sub(currentNode.T) > s.Duration {
			currentNode = currentNode.Next
			numNodesRemoved++
			continue
		}
		if currentNode == nil { // means we should shake off everything
			s.OldestLogNode = nil
			s.LatestLogNode = nil
			break
		}

		// If we reach a point where the current node is within 2 mins of latest node, cut of everything below it
		if currentNode.Previous != nil { // remove the next reference from the previous node
			currentNode.Previous.Next = nil
		}

		currentNode.Previous = nil // remove the previous reference from the current node

		// this should break the chain
		s.OldestLogNode = currentNode
		break
	}

	return numNodesRemoved, nil

}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  - A L E R T S  -  S U B O B J E C T S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type AlertLogNode struct {
	LogMessageStructured
	Previous *AlertLogNode
	Next     *AlertLogNode
}

type Alert struct {
	Start time.Time
	End   time.Time
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  C O N S U M E R  S O U R C E  S E T T I N G S  -  A L E R T S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type AlertTypeSourceSettings struct {
	Key             string                         // what key should we look at to include in the alert count?
	ValueMutateFunc func(v string) (string, error) // logic to determine if the value should count
	Values          []string                       // if value after mutation matches these, we include it in the count
}

func NewAlertTypeSourceSettings(key string, valueMutateFuncName string, values []string) (AlertTypeSourceSettings, error) {
	var s AlertTypeSourceSettings
	s.Key = key

	// Populate right Mutator Func Field
	switch valueMutateFuncName {
	case "":
		s.ValueMutateFunc = nil
	case "HTTPStatusLineToSection":
		s.ValueMutateFunc = HTTPStatusLineToSection
	default:
		return s, fmt.Errorf("value mutate func name '%s' not recognized", valueMutateFuncName)
	}

	s.Values = values

	return s, nil
}

func (s AlertTypeSourceSettings) GetKey() string {
	return s.Key
}

func (s AlertTypeSourceSettings) GetCleanedValue(msg LogMessageStructured) (string, error) {
	// Get the Key that we're looking for in the log message for this Stats
	// Once we have the key, get the value for that key e.g. key could be "host" and the value could  be  "100.0.0,1"
	var val = msg.GetValue(s.Key)

	// But sometimes we can't get have value as is, we need to clean it up for stats purpose e.g. extract "/api" from "GET /api/users HTTP1/0"
	if s.ValueMutateFunc != nil {
		var err error
		val, err = s.ValueMutateFunc(val)
		if err != nil {
			return val, err
		}
	}
	return val, nil
}

func (s AlertTypeSourceSettings) GetCleanedKV(msg LogMessageStructured) (map[string]string, error) {
	return msg.KV, nil
}

func (s AlertTypeSourceSettings) IsMatch(msg LogMessageStructured) (bool, error) {
	// If key is empty here, this means that we don't care about the key and want to count everything
	if s.GetKey() == "" {
		return true, nil
	}

	// Once we have the key, get the value for that key e.g. key could be "host" and the value could  be  "100.0.0,1"
	// But sometimes we can't get have value as is, we need to clean it up for stats purpose e.g. extract "/api" from "GET /api/users HTTP\1.0"
	value, err := s.GetCleanedValue(msg)
	if err != nil {
		return false, err
	}

	for _, v := range s.Values {
		if value == v {
			return true, nil
		}
	}

	return false, nil
}
