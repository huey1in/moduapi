package process

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBoundedPrivateDiagnostics(t *testing.T) {
	tail := newStderrTail()
	for _, line := range []string{`Authorization: Bearer synthetic-token`, `{"event":"failed","code":"raw-affinity-synthetic"}`, `{"event":"failed","body":"synthetic body"}`, `Cookie: synthetic-cookie`, `https://synthetic:secret@example.invalid`, `{"event":"ready"} trailing`, strings.Repeat("x", 100000)} {
		tail.Write([]byte(line + "\n"))
	}
	if tail.String() != "" {
		t.Fatal("free-form material retained")
	}
	tail.Write([]byte(`{"event":"rea`))
	tail.Write([]byte("dy\"}\n"))
	if !strings.Contains(tail.String(), "ready") {
		t.Fatal("fragmented record lost")
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 1000 {
				tail.Write([]byte("{\"event\":\"failed\",\"code\":\"timeout\"}\n"))
				_ = tail.String()
			}
		})
	}
	wg.Wait()
	if len(tail.String()) > StderrTailBytes || len(tail.line) > stderrLineBytes {
		t.Fatal("unbounded diagnostics")
	}
}
func TestBackoffBounds(t *testing.T) {
	o := DefaultRestartOptions()
	for n := 1; n < 20; n++ {
		for range 100 {
			d := o.delay(n)
			if d < o.Initial*8/10 || d > o.Max {
				t.Fatal(d)
			}
		}
	}
	o.Initial = time.Second
	o.Max = 2 * time.Second
	o.Jitter = 0
	if o.delay(2) != 2*time.Second || o.delay(100) != 2*time.Second {
		t.Fatal("not capped exponential")
	}
}
