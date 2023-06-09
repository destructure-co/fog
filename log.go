package fog

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"sync"
	"time"

	"github.com/lucasb-eyer/go-colorful"
)

// LogMux is a log multiplexer.
// It accepts writes for multiple registered log streams and merges the output.
//
// Lines for a given stream are prefixed with the name of the stream and a
// color. Interleaving of streams is minimized as much as possible.
type LogMux struct {
	mu      sync.Mutex
	streams map[string]*LogStream
	bufMs   time.Duration
	isDirty bool
	w       io.Writer
}

// LogStream is an individual log stream of the multiplexer.
type LogStream struct {
	// the stream's name
	name string
	// the color to use for log entries
	clr color.Color
	// the write callback
	w func(p []byte) (int, error)
}

// NewLogMux allocates and returns a new LogMux.
func NewLogMux(w io.Writer) *LogMux {
	return &LogMux{
		streams: make(map[string]*LogStream),
		bufMs:   time.Millisecond * 10,
		w:       w,
	}
}

// Stream adds an additional log stream to the multiplexer.
//
// Stream panics if a stream has already been registered with the given name.
func (m *LogMux) Stream(name string) *LogStream {
	clr := colorful.FastHappyColor()

	// TODO: move more of this into helper fns
	// TODO: refresh colors before ever writing
	var b bytes.Buffer
	var t *time.Timer

	w := func(p []byte) (int, error) {
		b.Write(p)

		if t == nil {
			t = time.AfterFunc(m.bufMs, func() {
				// TODO: include style info
				// TODO: only flush full lines when possible
				// TODO: should we return this return value?

				m.w.Write(b.Bytes())
				b.Reset()
				t = nil
			})
		}

		return len(p), nil
	}

	s := &LogStream{
		name: name,
		clr:  clr,
		w:    w,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.streams[name]

	if exists {
		panic(fmt.Errorf("Stream %s already exists", name))
	}

	m.streams[name] = s
	m.isDirty = true

	return s
}

// refresh the colors of the streams to re-distribute them across the color space.
func (m *LogMux) refreshColors() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isDirty {
		return
	}

	pal := colorful.FastHappyPalette(len(m.streams))

	i := 0
	for _, v := range m.streams {
		v.clr = pal[i]
		i++
	}

	m.isDirty = false
}

// Write implements io.Writer for a log stream.
func (s *LogStream) Write(p []byte) (int, error) {
	return s.w(p)
}
