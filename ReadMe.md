# LogDog

## Introduction

Logdog is a log ingestor built for fun. It is an executable program that can ingest log data and carry out some real-time analysis on the data, showing statistics and alerts on it.

### Entities

 There are three major entities in the design of this application. 
 
 1) **LogSource**:
    LogSource defines a single source of log data. Multiple log sources of different settings can used in parallel together. 
 
    The implementation allows for high configurability around the format of the log source, such as different header keys, header key that represents timestamp, format of timestamp (UNIX vs. else), csv vs. others etc. Log Sources can be defined and configured through the config file, which also includes some documentation on config level settings.
    
    Each log source basically provides an io.Reader interface which is used to read log line by line, and functionality to understand and parse a log text.
 
 2) **Queue & Processor**: 
    Queue is a buffered channel that is used to communicate between the LogSources that are streaming log data, and the listener which takes a raw log line and processes it (LogProcessor). Each of these things is running in it's own gorountine. 
    
    Once a raw log line is received by a log processor, through the queue, the processor parses the log text to extract the timestamp and other key-value pairs. The settings for the particular LogSource of this log message is used to determine how to parse the log. Once parsed, the LogMessage is now a StructuredLogMessage - and it can be forwarded to LogConsumers.
 
3) **LogConsumer**:
	A LogConsumer is an entity that can consume a structured log and do interesting things with it. A Log Consumer broadcasts a channel to the outside, and that channel can be used to send Structured Log Messages to Log Consumer, which can then process it accordingly.
    
    We have two kinds of Log Consumers implemented in this project: Stats & Alerts.	
    - **Stats**:
        Stats keep track/counts of occurrence of certain key-values in the log, and and the counts are dumped periodically according to the configured duration. Stats are configured through the config file. 

        Stats store the counts over discrete _n_ second windows, and once the window is over (+ a few seconds), the stats for that window are printed on Stdin. Only the top 5 most occurring are printed in  descended order.

        ```
        [Section most hits] Stats Report:
	    Time Start: 2019-02-07 21:18:00 +0000 UTC
	    Time End  : 2019-02-07 21:18:10 +0000 UTC
		    /api	:	10
                Breakdown by status
                    200	:	8
                    500	:	1
                    404	:	1
                Breakdown by remotehost
                    "10.0.0.2"	:	1
                    "10.0.0.3"	:	1
                    "10.0.0.5"	:	1
                    "10.0.0.1"	:	7
                Breakdown by authuser
                    "apache"	:	10
            /report	:	10
                Breakdown by remotehost
                    "10.0.0.3"	:	2
                    "10.0.0.1"	:	4
                    "10.0.0.2"	:	2
                    "10.0.0.5"	:	2
                Breakdown by authuser
                    "apache"	:	10
                Breakdown by status
                    500	:	1
                    200	:	9 
        ```

    - **Alerts**: 
        Alert types keep 'rolling' track of a count of events, and notifications can be triggered if counts exceed a certain pre-configured threshold over a given period of time. Alerts are automatically if the rolling-count becomes lower than the threshold again. Alert types are also configured in the config file. 

        Alerts carry out their functionality by  maintaining a dynamic, ordered linked-list of log messages. Each new log message is put in the linked list according to its timestamp. This gives us an easy way to find the number of logs for last _n_ seconds. Old nodes in this list are automatically removed.

        Sample alerts:
        ```
        [NOTICE] High traffic generated an alert - hits = 60, triggered at 2019-02-07 21:11:07 +0000 UTC
        [NOTICE] High traffic alert recovered at 2019-02-07 21:13:09 +0000 UTC
        [NOTICE] High traffic generated an alert - hits = 60, triggered at 2019-02-07 21:15:31 +0000 UTC
        [NOTICE] High traffic alert recovered at 2019-02-07 21:17:09 +0000 UTC
        ```


### Flow

In short, all we're doing is: 1) Read Log Message (a single log line) from Log Sources (e.g. a particular csv file). 2) Push each Log Message into a Queue (buffered channel). 3) Processor picks up the Log Message from the Queue, and parses parse it (extract timestamp, key-value pairs etc.) and converts it into a Structured Log Message. 4) It then sends the Structured Log Message to the channels of all Log Consumers (stats, alerts handlers) that want to consume this Log Message. 5) Log Consumers handle the Log Message, keep temporary counts of things to do things like printing periodic stats, alerts etc.

![Flow Diagram](https://i.imgur.com/sp5i2iJ.png)

## Getting Started

### Running

The easiest way to get the application up running is by calling

    make run-docker

The above command assumes you have docker setup. The other way to get the application running is to build and run it locally using:

    make run-local
    
The above command assumes that you have Go 1.13 installed. You may need to run `make setup-local` to install Go dependencies.

### Configuration File

Most of the application: log sources, size of buffered channel, types of stats, alerts is configured in the configuration file. The file included in this project has comments/documentation explaining the use of the config file. It can be accessed [here](config.toml).

## Next Steps

There are quite a few things I would like to do if I am able to spend more time on it. To list a few:

- Smarter Stats: I couldn't time to play around more with it.

- Tighter Control over Concurrency: Implement tighter control over concurrency so I can scale according to the load. 

- Concurrency within Log Consumers: Introduce concurrency within Log Consumers. They are most processing heavy parts of the system right now. Even though I have decoupled them with the rest of the system using buffered channels, they can still slow things down. I implemented locks while building them but never got the chance to actually process multiple logs at once per consumer. 

- Test Coverage: The test coverage right now is 48%, which is much lower than my ideal aim of 90%. I would probably want to write more unit tests around the channels.

- Refactor Alerts and Stats: Alerts and Stats logic was the last thing I implemented, and I feel I would want to refactor it:
    - break the functionality down into smaller testable functions
    - implement garbage collection in Stats code as right we're storing stats for all the logs. Alerts already  has a garbage collector (kind of) implemented as it was more important there.


