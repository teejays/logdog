// package main is an executable program that can ingest log data through multiple source, and can show statistics and alerts
// on it.
// There are four major entities in the design on this application.
// 1. LogSource:	Defines a source of log data, which can be configured to include different format or source e.g csv from file,
// 					tsv from std.in etc. Each log source provides an io.Reader interface which is used to read log line by line in
// 					a separate goroutine. Each line read from LogSource is sent through the Queue, and is collected by LogProcessor.
//					Log Sources can be defined and configured through the config file, which also includes some documention on different
// 					config level settings.
//
// 2. Queue:		Queue is a buffered channel that is used to communicate between the goroutines that are streaming log lines, and the
//					goroutine which is listening for log lines for processing. The capacity of this buffered channel can be controlled
//					through the config file. Once a log is received by a log processor, which listens on this channel, it parses the log
//					text to extract the timestamp and other key-value pairs. The settings for the particular LogSource of this log message
//					is used to determine how to parse the log. Once parsed, the LogMessage is now a StructuredLogMessage - and it can
//					be forwarded to LogConsumers.
//
// 3. LogConsumer:	A LogConsumer is an entity that can consume a structured log and do interesting things with it. We have two kinds of
//					Log Consumers implemented in this project: Stats & Alerts.
//	- Stats:		Stats types are special kind of LogConsumers that can be configured to keep track/count certain key-values of the log, and
//					and the counts can be dumped periodically according to the configured duration. Stats can be defined and configured in
//					the config file.
//  - Alerts:		Alert types are LogConsumers that can be configured to keep track an rolling count of events, and notifications can be triggered
//					if counts exceed a certain pre-configured threshold over a given period of time. Alerts are automatically closed
//					if the rolling-count becomes lower than the threshold again. Alert types can be defined  and configured in the config.
//
// In short, all we're doing is :
// 1) Read log lines from LogSources and push them in a Queue (buffered channel).
// 2) Detect the log from the channel, use the information from LogSource of log to parse the log line (extract timestamp etc.).
// 3) Send the structured log to LogConsumers (Stats, Alerts) and let their handlers handle the log message.
// 4) LogConsumers, given the log messages, keep some in-memory data on the log, which they use to provide notifications.
//
// [Source A, Source B...] ===[queue: shared channel]===> [Log Processor] ===[multiple consumer channels]===> [Consumer A, Consumer B...]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/teejays/logdoc/config"

	"github.com/teejays/clog"
)

// Args holds the command line arguments that are needed to run the application
type Args struct {
	// ConfigFilePath is the file path where config file for this application lives
	ConfigFilePath string
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  Q U E U E
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// CreateQueue creates a go channel that is used to send log messages from
// sources to the processor.
func CreateQueue(size int) chan LogMessage {
	var q = make(chan LogMessage, size)
	return q
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  M A I N
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

// run is same as main, but allows us to return an error.
func run() error {

	// Step 1: Initialize the Command Line args & Read the Config file
	var args Args
	flag.StringVar(&args.ConfigFilePath, "config-file", "", "Path to the config file in TOML format (required)")
	flag.Parse()

	cfg, err := config.ReadConfigTOML(args.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("uploading config file at %s: %w", args.ConfigFilePath, err)
	}

	// Step 2: From the config file, create LogSource instances
	var sources []LogSource
	for _, cfgSrc := range cfg.LogSources {

		if cfgSrc.Disabled {
			continue
		}

		// Get the LogSource instance
		src, err := NewLogSourceFromConfig(cfgSrc)
		if err != nil {
			return fmt.Errorf("creating log source '%s': %w", cfgSrc.Name, err)
		}

		// Store the LogSource instance in-memory for shared access
		err = RegisterSourceInStore(src)
		if err != nil {
			return err
		}

		sources = append(sources, src)
	}

	if len(sources) < 1 {
		return fmt.Errorf("No valid log sources created")
	}
	clog.Debugf("Log Sources: %v", sources)

	// Step 3: Create a channel that can be used to push messages from log sources to the listener
	var queue = CreateQueue(cfg.InQueueBufferSize)

	// - Start the Log Listener in a goroutine
	go ListenToLogSources(queue)

	// Step 4: Setup the log consumers.
	var consumers []LogConsumer

	// - Register Stats types: these define what kind of stats do keep track of
	clog.Debugf("Stats Types: %v", cfg.Stats.Types)
	for _, st := range cfg.Stats.Types {
		if st.Disabled {
			continue
		}

		// Get the Stats LogConsumer instance
		c, err := NewStatsTypeFromConfig(st)
		if err != nil {
			return err
		}

		// Store the LogConsumer in memory for shared access
		err = RegisterConsumerInStore(c)
		if err != nil {
			return err
		}

		consumers = append(consumers, c)
	}

	// - Register Alert Types: these define what kind of alerts do we keep track of
	clog.Debugf("Alert Types: %v", cfg.Alert.Types)
	for _, at := range cfg.Alert.Types {
		if at.Disabled {
			continue
		}

		// Get the Alert LogConsumer instance
		c, err := NewAlertTypeFromConfig(at)
		if err != nil {
			return err
		}

		// Store the LogConsumer in memory for shared access
		err = RegisterConsumerInStore(c)
		if err != nil {
			return err
		}

		consumers = append(consumers, c)
	}

	// - For each LogConsumer, we need to start a listener go routine that received log messages for them
	for _, c := range consumers {
		go func(c LogConsumer) {
			err := ListenForLogMessageOnConsumer(c)
			if err != nil {
				clog.Warnf("[Consumer %s] Listening for messages: %s", c.GetName(), err)
			}

		}(c)
	}

	// Step 6: Initialize and start reading from all the log sources.
	// At this point all we need to do is start steaming log from source. We already
	//  have a listener listening to take raw log data, process it a little, and send to consumers.

	// WaitGroup helps making sure that we don't exit the program unless all log sources are over
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(src LogSource) {
			defer func() {
				if r := recover(); r != nil {
					clog.Errorf("[Recovered Panic] streaming log from source '%s': %s", src.GetName(), r)
				}
				wg.Done()
			}()
			err := StreamLogMessagesFromSource(src, queue)
			if err != nil {
				clog.Errorf("Initializing log source %s: %w", src.GetName(), err)
			}
		}(src)
	}

	wg.Wait()

	clog.Info("Exiting.")

	return nil
}

func StreamLogMessagesFromSource(src LogSource, inQueue chan LogMessage) error {

	reader, err := src.NewReader()
	if err != nil {
		return err
	}

	buffReader := bufio.NewReader(reader)

	var id int64
	for {
		// Read the next/first line
		text, err := buffReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		// End of stream
		if err == io.EOF {
			clog.Debugf("[%s] Stream EOF: %s", src.GetName(), err)
			break
		}

		// Text has '\n' at the end, which we should remove
		text = StripTrailingNewlineCharacter(text)

		// Special cases
		if text == "\\q" {
			clog.Debugf("[%s] Exit signal detected", src.GetName())
			break
		}

		// For some Log Sources, the first line is a header. We need to save the header info in the LogSource store.
		if id == 0 && src.GetSettings().UseFirstlineAsHeader {
			clog.Debugf("[%s] First line is header: %s", src.GetName(), text)

			// Split the headers line into individual parts. The LogSource knows how to do that
			srcSettings := src.GetSettings()
			srcSettings.Headers = srcSettings.Format.GetPartsFromText(text, true)
			src.SetSettings(srcSettings)

			err := SetSourceInStore(src)
			if err != nil {
				return fmt.Errorf("could not update source '%s' with headers info", src.GetName())
			}
			id++
			continue
		}

		// Uniqueinternal ID for this log message
		id++

		// clog.Debugf("[%s] [%d] Sending message to queue: %s", src.GetName(), id, text)
		inQueue <- LogMessage{SourceName: src.GetName(), Message: text, Id: id}
	}

	reader.Close()

	return nil
}

func ListenToLogSources(inQueue chan LogMessage) error {

	for {
		rawMsg := <-inQueue

		if rawMsg.IsCancelSignal {
			clog.Warnf("[%s] [%d] Cancel Signal Received", rawMsg.SourceName, rawMsg.Id)
			break
		}

		clog.Debugf("[%s] [%d] Message received from queue: %s", rawMsg.SourceName, rawMsg.Id, rawMsg.Message)

		// If an empty message, do nothing
		if strings.TrimSpace(rawMsg.Message) == "" {
			return fmt.Errorf("received an empty message")
		}

		// Make the Log Message Structured
		// Get the format config for this source type
		src, err := GetSourceFromStore(rawMsg.SourceName)
		if err != nil {
			return err
		}
		settings := src.GetSettings()
		clog.Debugf("[%s] [%d] Source settings fetched: %+v", rawMsg.SourceName, rawMsg.Id, settings)

		msg, err := NewLogMessageStructured(rawMsg, settings)
		if err != nil {
			return err
		}

		clog.Debugf("[%s] [%d] Structured Log Message created", rawMsg.SourceName, rawMsg.Id)

		// Get all the consumers for this source...
		// Get all the consumer's channels
		// Send it to the channels

		consumers := GetConsumersBySourceFromStore(msg.SourceName)
		clog.Debugf("[%s] [%d] # Consumers Fetched: %d", rawMsg.SourceName, rawMsg.Id, len(consumers))

		for _, c := range consumers {
			clog.Debugf("[%s] [%d] Handling Consumer: %s", rawMsg.SourceName, rawMsg.Id, c.GetName())
			ch := c.GetChannel()
			ch <- msg
		}

	}

	return nil

}

func ListenForLogMessageOnConsumer(c LogConsumer) error {

	// Listen on the
	ch := c.GetChannel()

	firstMsg := true
	for {
		clog.Debugf("[Consumer %s] Waiting for message...", c.GetName())
		msg := <-ch
		clog.Debugf("[%s] [%d] [%s] Message received: %+v", msg.SourceName, msg.Id, c.GetName(), msg)
		// Prepare for consumption (call it on first message)
		if firstMsg {
			err := c.PrepareForConsumption(msg.T)
			if err != nil {
				return err
			}
			firstMsg = false
		}

		if msg.IsCancelSignal {
			clog.Warnf("[%s] [%d] [%s] Cancel Signal Received", msg.SourceName, msg.Id, c.GetName())
			break
		}

		err := c.ConsumeLog(msg)
		if err != nil {
			clog.Errorf("[%s] [%d] [%s] Error consuming log: %s", msg.SourceName, msg.Id, c.GetName(), err)
			continue
		}
	}

	return nil
}
