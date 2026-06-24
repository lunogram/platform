package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
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

func NewEncoder(ctx context.Context, w http.ResponseWriter) *Encoder {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	enc := &Encoder{
		ResponseWriter: w,
		flusher:        w.(http.Flusher),
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := enc.WriteComment("keepalive"); err != nil {
					return
				}
			}
		}
	}()

	return enc
}

type Encoder struct {
	mu sync.Mutex
	http.ResponseWriter
	flusher http.Flusher
}

func (encoder *Encoder) WriteEvent(event string, data any) error {
	jsonBytes, _ := json.Marshal(data)

	encoder.mu.Lock()
	defer encoder.mu.Unlock()

	_, err := fmt.Fprintf(encoder.ResponseWriter, "event: %s\n", event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(encoder.ResponseWriter, "data: %s\n\n", jsonBytes)
	if err != nil {
		return err
	}
	encoder.flusher.Flush()
	return nil
}

func (encoder *Encoder) WriteComment(text string) error {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()

	_, err := fmt.Fprintf(encoder.ResponseWriter, ": %s\n\n", text)
	if err != nil {
		return err
	}
	encoder.flusher.Flush()
	return nil
}

func (encoder *Encoder) Write(p []byte) (int, error) {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()
	return encoder.ResponseWriter.Write(p)
}
