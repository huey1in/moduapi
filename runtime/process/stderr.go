package process

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

const StderrTailBytes = 64 << 10
const stderrLineBytes = 4096

// Free-form stderr cannot be redacted reliably (a raw session key or body may
// have no identifying label). Retain only closed-vocabulary diagnostic records.
// This intentionally favors confidentiality over arbitrary traceback capture.
type stderrTail struct {
	mu         sync.Mutex
	tail, line []byte
	overflow   bool
}

func newStderrTail() *stderrTail {
	return &stderrTail{tail: make([]byte, 0, StderrTailBytes), line: make([]byte, 0, stderrLineBytes)}
}
func (s *stderrTail) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(b)
	for _, c := range b {
		if c == '\n' {
			if !s.overflow {
				s.record(s.line)
			}
			s.line = s.line[:0]
			s.overflow = false
			continue
		}
		if len(s.line) >= stderrLineBytes {
			s.overflow = true
		}
		if !s.overflow {
			s.line = append(s.line, c)
		}
	}
	return n, nil
}
func (s *stderrTail) record(line []byte) {
	var rec struct {
		Event string `json:"event"`
		Code  string `json:"code,omitempty"`
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&rec) != nil || d.Decode(new(any)) != io.EOF {
		return
	}
	switch rec.Event {
	case "startup", "ready", "draining", "stopped", "failed":
	default:
		return
	}
	switch rec.Code {
	case "", "config_rejected", "transport_error", "protocol_error", "cancelled", "timeout", "internal_error", "crash":
	default:
		return
	}
	b, _ := json.Marshal(rec)
	b = append(b, '\n')
	if excess := len(s.tail) + len(b) - StderrTailBytes; excess > 0 {
		copy(s.tail, s.tail[excess:])
		s.tail = s.tail[:len(s.tail)-excess]
	}
	s.tail = append(s.tail, b...)
}
func (s *stderrTail) String() string { s.mu.Lock(); defer s.mu.Unlock(); return string(s.tail) }
func (p *Process) StderrTail() string {
	if p.stderr == nil {
		return ""
	}
	return p.stderr.String()
}
