package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func NewDecoder(res *http.Response) *Decoder {
	if res == nil || res.Body == nil {
		return nil
	}

	scanner := bufio.NewScanner(res.Body)
	return &Decoder{rc: res.Body, scn: scanner}
}

type Event struct {
	Type string
	Data []byte
}

// A base implementation of a Decoder for text/event-stream.
type Decoder struct {
	evt Event
	rc  io.ReadCloser
	scn *bufio.Scanner
	err error
}

func (s *Decoder) Next() bool {
	if s.err != nil {
		return false
	}

	event := ""
	data := bytes.NewBuffer(nil)

scanner:
	for s.scn.Scan() {
		txt := s.scn.Bytes()

		// Dispatch event on an empty line
		if len(txt) == 0 {
			s.evt = Event{
				Type: event,
				Data: data.Bytes(),
			}
			return true
		}

		// Split a string like "event: bar" into name="event" and value=" bar".
		name, value, _ := bytes.Cut(txt, []byte(":"))

		// Consume an optional space after the colon if it exists.
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch string(name) {
		case "":
			// An empty line in the for ": something" is a comment and should be ignored.
			continue
		case "event":
			event = string(value)
		case "data":
			_, s.err = data.Write(value)
			if s.err != nil {
				break scanner
			}
			_, s.err = data.WriteRune('\n')
			if s.err != nil {
				break scanner
			}
		}
	}

	if s.scn.Err() != nil {
		s.err = s.scn.Err()
	}

	return false
}

func (s *Decoder) Event() Event {
	return s.evt
}

func (s *Decoder) Close() error {
	return s.rc.Close()
}

func (s *Decoder) Err() error {
	return s.err
}

func NewEncoder(w http.ResponseWriter) *Encoder {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	return &Encoder{
		ResponseWriter: w,
		flusher:        w.(http.Flusher),
	}
}

type Encoder struct {
	http.ResponseWriter
	flusher http.Flusher
}

func (encoder *Encoder) WriteEvent(event string, data any) {
	jsonBytes, _ := json.Marshal(data)
	fmt.Fprintf(encoder, "event: %s\n", event)
	fmt.Fprintf(encoder, "data: %s\n\n", jsonBytes)
	encoder.flusher.Flush()
}

func (encoder *Encoder) Write(p []byte) (int, error) {
	return encoder.ResponseWriter.Write(p)
}
