package main

import (
	"fmt"
	"sync"
)

/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
*  S T A T S  T Y P E  -  S T O R E
* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

// ConsumerStore holds an easy mapping of LogSources to LogConsumers that  are relevant to that
// LogSource. One source can have many different consumers acting on it. It is good for concurrent access.
type ConsumerStore struct {
	Data map[string][]LogConsumer
	Lock sync.RWMutex
}

var consumerStore ConsumerStore

func RegisterConsumerInStore(c LogConsumer) error {
	consumerStore.Lock.Lock()
	defer consumerStore.Lock.Unlock()

	// If map wasn't initialized (i.e. first element coming), initialize it
	if consumerStore.Data == nil {
		consumerStore.Data = make(map[string][]LogConsumer)
	}

	allSrcSettings := c.GetAllSourceSettings()
	if len(allSrcSettings) < 1 {
		return fmt.Errorf("no valid source settings for consumer")
	}

	for srcName, _ := range allSrcSettings {
		consumerStore.Data[srcName] = append(consumerStore.Data[srcName], c)
	}

	return nil
}

func GetConsumersBySourceFromStore(srcName string) []LogConsumer {
	consumerStore.Lock.RLock()
	defer consumerStore.Lock.RUnlock()

	return consumerStore.Data[srcName]
}
