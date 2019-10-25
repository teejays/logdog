package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/teejays/logdoc/config"
)

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// LogSource defines a single source of log ingestion, which can be a file (csv, or other format), Std. In, or anything that can
// generated an io.Reader stream. This has been implemented as an interface to allow for handling various types sources, which can differ
// considerably in how they are handled.
type LogSource interface {
	GetName() string
	GetSettings() LogSourceSettings
	SetSettings(LogSourceSettings)
	NewReader() (io.ReadCloser, error)
}

// NewLogSource takes in all the required info necessary to create and return the desired type of LogSource
func NewLogSourceFromConfig(req config.ConfigLogSource) (LogSource, error) {

	// srcConfigFormat := srcConfig.Format
	var src LogSource

	srcSettings, err := NewLogSourceSettingsFromConfig(req.Settings)
	if err != nil {
		return nil, err
	}

	switch req.Type {
	case "file":
		src, err = NewFileSource(req.Name, srcSettings, req.Path)
		if err != nil {
			return nil, err
		}

	case "stdin":
		src, err = NewStdInSource(req.Name, srcSettings)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized config source type found: %s", req.Type)
	}

	return src, nil

}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  F I L E
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// FileSource is an implementation of LogSource. It handles logs from a file.
type FileSource struct {
	filePath string
	*baseLogSource
}

// NewFileSource generates and returns a new instance of FileSource implementation of a LogSource interface.
func NewFileSource(name string, settings LogSourceSettings, filePath string) (LogSource, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("file path is empty")
	}
	var src FileSource
	src.filePath = filePath
	src.baseLogSource = &baseLogSource{name: name, settings: settings}
	return src, nil
}

// NewReader provides a byte stream for the FileSource
func (src FileSource) NewReader() (io.ReadCloser, error) {
	// Open the file
	file, err := os.Open(src.filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	return file, nil
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  S T D .  I N
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// StdIn is an implementation of LogSource. It handles logs coming directly from Standard In.
type StdInSource struct {
	*baseLogSource
}

// NewStdInSource generates and returns a new instance of StdIn implementation of a LogSource interface.
func NewStdInSource(name string, settings LogSourceSettings) (LogSource, error) {
	var src StdInSource
	src.baseLogSource = &baseLogSource{name: name, settings: settings}
	return src, nil
}

// NewReader provides a byte stream for the StdInSource.
func (StdInSource) NewReader() (io.ReadCloser, error) {
	return os.Stdin, nil
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  B A S E  I M P L E M E N T A T I O N
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// baseLogSource is a struct that partially implements LogSource interface. It implements the functions that are going be common
// among any LogSource implementation. While implementing a LogSource, we can now just embed (pseudo-inherit) this struct so we
// don't have to implement these functions repeatedly.
type baseLogSource struct {
	name     string
	settings LogSourceSettings
}

// GetName returns the name of the LogSource
func (src *baseLogSource) GetName() string {
	return src.name
}

// GetSettings returns the name LogSourceFormat of the LogSource. See LogSourceFormat for more info.
func (src *baseLogSource) GetSettings() LogSourceSettings {
	return src.settings
}

// SetSettings returns the name LogSourceFormat of the LogSource. See LogSourceFormat for more info.
func (src *baseLogSource) SetSettings(settings LogSourceSettings) {
	src.settings = settings
	return
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  S E T T I N G S
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// LogSourceSettings contains the information necessary to process logs coming from a particular LogSource.
type LogSourceSettings struct {
	Format               LogSourceFormat
	Headers              []string
	TimestampKey         string
	TimestampFormat      TimestampFormat // SourceTimestampType
	UseFirstlineAsHeader bool
}

// NewLogSourceSettingsFromConfig takes all the config representation of log source settings and creates an instance of LogSourceSettings.
// It does the heavy lifting of determing what LogSourceFormat, TimestampFormat to use.
func NewLogSourceSettingsFromConfig(req config.ConfigLogSourceSettings) (LogSourceSettings, error) {

	srcConfig := LogSourceSettings{
		Headers:              req.Headers,
		TimestampKey:         req.TimestampKey,
		UseFirstlineAsHeader: req.UseFirstlineAsHeader,
	}

	// Format Type
	switch req.Format {
	case "csv":
		srcConfig.Format = LogSourceFormat_CSV{}
	default:
		return srcConfig, fmt.Errorf("LogSource format '%s' is not recognized", req.Format)
	}

	// Time Format Type
	switch req.TimestampFormat {
	case "unix":
		srcConfig.TimestampFormat = TimestampFormat_Unix{}
	default:
		return srcConfig, fmt.Errorf("source format timestamp format '%s' is not recognized", req.TimestampFormat)
	}

	return srcConfig, nil
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  F O R M A T
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// LogSourceFormat contains functionality necessary to understand and parse log text from a given LogSource. It
// is implemented as an interface because different format (e.g. csv, tsv, json etc.) need to be implemented in
// a different way.
type LogSourceFormat interface {
	// GetName returns the identifier of the given LogSourceFormat.
	GetName() string
	// GetPartsFromText takes a log string (single line) and split into individual parts. if stripQuotes is set to true, it
	// strips double quotes if they surround any of the split elements.
	GetPartsFromText(text string, stripQuotes bool) []string
	// GetKeyValueMap takes a log string (single line), information on headers, and converts into a key value map.
	GetKeyValueMap(text string, headers []string) (map[string]string, error)
}

// LogSourceFormat_CSV implements the LogSourceFormat interface. It handles the CSV format.
type LogSourceFormat_CSV struct{}

// GetName returns the identifier of the given LogSourceFormat.
func (s LogSourceFormat_CSV) GetName() string {
	return "csv"
}

// GetPartsFromText takes a log string (single line) and split into individual parts. if stripQuotes is set to true, it
// strips double quotes if they surround any of the split elements.
func (s LogSourceFormat_CSV) GetPartsFromText(text string, stripQuotes bool) []string {
	parts := strings.Split(text, ",")
	if stripQuotes {
		for i, p := range parts {
			parts[i] = removeQuotes(p)
		}
	}
	return parts
}

// GetKeyValueMap takes a log string (single line), information on headers, and converts into a key value map.
func (s LogSourceFormat_CSV) GetKeyValueMap(text string, headers []string) (map[string]string, error) {
	parts := s.GetPartsFromText(text, false)
	if len(parts) != len(headers) {
		return nil, fmt.Errorf("number of elements in the log line are different than number of headers: expected %d, got %d", len(headers), len(parts))
	}
	kv := make(map[string]string)
	for i, h := range headers { // the order of headers and data should be the same
		kv[h] = parts[i]
	}
	return kv, nil
}

/* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * *
*  L O G   S O U R C E  -  T I M E S T A M P
* * * * * * * * *  * * * * * * * * * * * * * * * * * * * * * * * */

// Timestamp format includes functionality to parse to string that represents time.
type TimestampFormat interface {
	// GetName returns an identifier for the given TimestampFormat.
	GetName() string
	// Parse takes the string that represents time, and parses into time.Time type.
	Parse(str string) (time.Time, error)
}

// TimestampFormat_Unix implements TimestampFormat interface, and handles the UNIX timestamp representation of time.
type TimestampFormat_Unix struct{}

// GetName returns an identifier for the given TimestampFormat.
func (s TimestampFormat_Unix) GetName() string {
	return "unix"
}

// Parse takes the string that represents time, and parses into time.Time type.
func (s TimestampFormat_Unix) Parse(str string) (time.Time, error) {
	var t time.Time
	// Convert unix seconds to int64
	unixTimestamp, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return t, fmt.Errorf("could not convert to unix timestamp: %w", err)
	}
	t = time.Unix(unixTimestamp, 0)
	return t, nil
}
