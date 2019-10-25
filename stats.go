package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/teejays/logdoc/config"

	"github.com/teejays/clog"
)

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  -  S T A T S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// StatsType implements a LogConsumer. It has the power to keep track of counts for periodic intervals.
type StatsType struct {
	Duration time.Duration
	baseLogConsumer

	Windows             []StatsWindow // Each window holds data on a given time-frame
	CurrentPointer      int           // points to the current window in the above Store
	QueuedNotifications []int
	LatestTimestamp     time.Time
}

func NewStatsTypeFromConfig(req config.ConfigStatsType) (*StatsType, error) {
	var c = StatsType{
		Duration: time.Duration(req.DurationSeconds * int64(time.Second)),
	}
	c.baseLogConsumer.Name = req.Name
	c.baseLogConsumer.Lock = &sync.RWMutex{}
	c.baseLogConsumer.InQueue = make(chan LogMessageStructured, 8)

	// Set Source Settings
	var sourceSettingsMap = make(map[string]LogConsumerSourceSettings)
	for _, cfgSettings := range req.SourceSettings {

		if _, exists := sourceSettingsMap[cfgSettings.Name]; exists {
			return nil, fmt.Errorf("multiple stats source settings for log source found")
		}

		settings, err := NewStatsTypeSourceSettings(cfgSettings.Key, cfgSettings.ValueMutateFuncName, cfgSettings.OtherKeys)
		if err != nil {
			return nil, err
		}

		sourceSettingsMap[cfgSettings.Name] = settings
	}

	c.baseLogConsumer.SourceSettings = sourceSettingsMap

	return &c, nil
}

func (c *StatsType) PrepareForConsumption(currentTime time.Time) error {
	// If StatsType has no time-windows, create some now...
	if len(c.Windows) < 1 {
		// Initialize it..
		clog.Debugf("[%s] Initializing StatsType.Windows...", c.Name)
		c.Windows = []StatsWindow{
			NewStatsWindow(time.Time{}, currentTime), // previous, dummy window since timeZero till current, in case we get a log that is from past
			NewStatsWindow(currentTime, currentTime.Add(c.Duration)),
		}
		c.CurrentPointer = 1
	}
	return nil
}

func (c *StatsType) ConsumeLog(msg LogMessageStructured) error {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	// By now, we have the value that we need to keep track of, we just need to add it to the right time window
	// Get current window that we have. If current window is not initialized, this means this is the first such message for this stats

	// Decide what window this log go to
	err := c.addToWindow(msg)
	if err != nil {
		return err
	}

	// Update latestTimestamp
	if c.LatestTimestamp.Before(msg.T) {
		c.LatestTimestamp = msg.T
	}

	// Release any notifications
	err = c.releaseNotifications()
	if err != nil {
		return err
	}

	return nil
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  S T A T S  -  F U N C T I O N S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

func (s *StatsType) addToWindow(msg LogMessageStructured) error {

	clog.Debugf("[%s] [%d] [%s] Finding the right counts window among %d windows", msg.SourceName, msg.Id, s.Name, len(s.Windows))
	windowIndex, newWindowCreated := s.determineWindow(msg.T)

	// We should send a notification if new window was created, because that means we have entered a new timeframe
	// But, let's notify after a few seconds so we have some lagging data as well. Hence, put it in a queue
	if newWindowCreated {
		s.QueuedNotifications = append(s.QueuedNotifications, s.CurrentPointer)
	}

	// Add the log to the stats window
	window := s.Windows[windowIndex]

	// If we reach this point, this means that we should add the log to the current stats window
	clog.Debugf("[%s] [%d] [%s] Window determined: index %d", msg.SourceName, msg.Id, s.Name, windowIndex)

	// Get the config of how to handle a log message from this source for this particular Stats
	srcSettings, err := s.GetSourceSettings(msg.SourceName)
	if err != nil {
		return err
	}
	cleanValue, err := srcSettings.GetCleanedValue(msg)
	if err != nil {
		return err
	}
	clog.Debugf("[%s] [%d] [%s] Value for key '%s' fetched: %s", msg.SourceName, msg.Id, s.Name, srcSettings.GetKey(), cleanValue)
	cleanKV, err := srcSettings.GetCleanedKV(msg)
	if err != nil {
		return err
	}

	window.add(cleanValue, cleanKV)

	s.Windows[windowIndex] = window

	// Make the pointer to the new window?
	s.CurrentPointer = windowIndex

	return nil
}

func (s *StatsType) determineWindow(t time.Time) (int, bool) {
	var newWindowCreated bool

	windowIndex := s.CurrentPointer

	for {
		// If we're in the future and haven't created a window for it yet
		if windowIndex >= len(s.Windows) {

			// If we're creating a new window, then let's print the stats
			newWindowCreated = true

			lastWindow := s.Windows[len(s.Windows)-1]

			newWindow := NewStatsWindow(lastWindow.End, lastWindow.End.Add(s.Duration))
			s.Windows = append(s.Windows, newWindow)
			clog.Debugf("Creating a new window from %s to %s", newWindow.Start, newWindow.End)

		}

		window := s.Windows[windowIndex]

		// if the log belongs to a previous window
		if t.Before(window.Start) {
			windowIndex--
			continue
		}

		// if the log belongs to a later window
		if t.After(window.End) || t.Equal(window.End) {
			windowIndex++
			continue
		}

		break

	}

	return windowIndex, newWindowCreated
}

func (st StatsType) getCurrentWindow() StatsWindow {
	return st.Windows[st.CurrentPointer]
}

func (s *StatsType) releaseNotifications() error {
	var doneIndexes []int
	for i := 0; i < len(s.QueuedNotifications); i++ {
		windowIndex := s.QueuedNotifications[i]
		statsWindow := s.Windows[windowIndex]
		if s.LatestTimestamp.Sub(statsWindow.End) > 2*time.Second {
			s.notify(windowIndex)
			doneIndexes = append(doneIndexes, i)
		}
	}
	for _, i := range doneIndexes {
		s.QueuedNotifications = append(s.QueuedNotifications[:i], s.QueuedNotifications[i+1:]...)
	}
	return nil
}

func (s StatsType) notify(windowIndex int) {
	statsWindow := s.Windows[windowIndex]

	// Print the stats in order, so get the order first
	var kCountMap = make(map[string]int)
	for k, stats := range statsWindow.StatsMap {
		kCountMap[k] = stats.Count
	}
	var orderedKeys = sortMapKeysByValue(kCountMap)
	clog.Noticef("kCountMap: %+v", kCountMap)
	clog.Noticef("OrderedKeys: %+v", orderedKeys)

	msg := fmt.Sprintf("[%s] Stats Report:\n\tTime Start: %s\n\tTime End  : %s\n", s.Name, statsWindow.Start, statsWindow.End)
	cnt := 0
	for _, k1 := range orderedKeys {
		// and only print top 5
		cnt++
		if cnt >= 5 {
			break
		}
		stats := statsWindow.StatsMap[k1]
		msg = msg + fmt.Sprintf("\t\t%s\t:\t%d\n", k1, stats.Count)
		for k2, v2 := range stats.OtherCounts {
			msg = msg + fmt.Sprintf("\t\t\tBreakdown by %s\n", k2)
			for k3, v3 := range v2 {
				msg = msg + fmt.Sprintf("\t\t\t\t%s\t:\t%d\n", k3, v3)
			}
		}
	}

	clog.Notice(msg)
	clog.Debugf("Window: %+v", statsWindow)
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G  C O N S U M E R  - S T A T S  -  S U B O B J E C T S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type StatsWindow struct {
	StatsMap map[string]Stats // map[value]counts
	Start    time.Time
	End      time.Time
}

type Stats struct {
	Count       int
	OtherCounts map[string]map[string]int // map[key][value]count e.g. [host] -> [100.0.0.1] -> 25
}

func NewStatsWindow(start, end time.Time) StatsWindow {
	return StatsWindow{
		StatsMap: make(map[string]Stats),
		Start:    start,
		End:      end,
	}
}

func (w *StatsWindow) add(value string, kv map[string]string) {

	// Find stats for right value
	stats := w.StatsMap[value]

	// Increment the counter
	stats.Count++
	// Increment counters for other keys
	if stats.OtherCounts == nil {
		stats.OtherCounts = make(map[string]map[string]int)
	}
	for k, v := range kv {
		if stats.OtherCounts[k] == nil {
			stats.OtherCounts[k] = make(map[string]int)
		}
		stats.OtherCounts[k][v]++
	}

	w.StatsMap[value] = stats

}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
* C O N S U M E R  S O U R C E  S E T T I N G S  -  S T A T S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

type StatsTypeSourceSettings struct {
	Key             string                         // what key in the log and we keeping a count by?
	ValueMutateFunc func(v string) (string, error) // do we need to do any processing on the key's value before we use it for count?
	OtherKeys       []string                       // KeysForSubCounts
}

func NewStatsTypeSourceSettings(key string, valueMutateFuncName string, otherKeys []string) (StatsTypeSourceSettings, error) {
	var s StatsTypeSourceSettings
	s.Key = key
	s.OtherKeys = otherKeys

	// Populate right Mutator Func Field
	switch valueMutateFuncName {
	case "":
		s.ValueMutateFunc = nil
	case "HTTPStatusLineToSection":
		s.ValueMutateFunc = HTTPStatusLineToSection
	default:
		return s, fmt.Errorf("value mutate func name '%s' not recognized", valueMutateFuncName)
	}

	return s, nil
}

func (s StatsTypeSourceSettings) GetKey() string {
	return s.Key
}

func (s StatsTypeSourceSettings) GetCleanedValue(msg LogMessageStructured) (string, error) {
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

func (s StatsTypeSourceSettings) GetCleanedKV(msg LogMessageStructured) (map[string]string, error) {
	cleanKV := filterMapCopy(msg.KV, s.OtherKeys)
	return cleanKV, nil
}

func (s StatsTypeSourceSettings) IsMatch(msg LogMessageStructured) (bool, error) {
	return true, nil
}
