// package main handles the run-time in-memory storage of the Logdog application. It is highly coupled with the rest of the packages.
package main

import (
	"fmt"
	"sync"
)

/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  S E T T I N G S - S T O R E
* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

// SourceStore is a cache of teh configs of LogSources. This is needed when processing data from a log source.
type SourceStore struct {
	Data map[string]LogSource
	Lock sync.RWMutex
}

var sourceStore SourceStore

func RegisterSourceInStore(src LogSource) error {
	sourceStore.Lock.Lock()
	if sourceStore.Data == nil {
		sourceStore.Data = make(map[string]LogSource)
	}
	sourceStore.Lock.Unlock()

	// Check if we've already registered the source type
	if IsSourceRegisteredInStore(src.GetName()) {
		return fmt.Errorf("source '%s' has already been registered", src.GetName())
	}

	// If not, register it
	return SetSourceInStore(src)
}

func SetSourceInStore(src LogSource) error {
	sourceStore.Lock.Lock()
	defer sourceStore.Lock.Unlock()

	sourceStore.Data[src.GetName()] = src
	return nil
}

func GetSourceFromStore(name string) (LogSource, error) {
	sourceStore.Lock.RLock()
	defer sourceStore.Lock.RUnlock()

	src, exists := sourceStore.Data[name]
	if !exists {
		return src, fmt.Errorf("source name '%s' not found in store", name)
	}
	return src, nil
}

func IsSourceRegisteredInStore(name string) bool {
	sourceStore.Lock.RLock()
	defer sourceStore.Lock.RUnlock()

	if _, exists := sourceStore.Data[name]; exists {
		return true
	}
	return false
}
