package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Validate the measurement, not a synthetic gateway: both response forms must
// carry upstream work durations, and SSE must reach the reader incrementally.
func TestMockTimingAndIncrementalDelivery(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(mock))
	defer s.Close()
	for _, stream := range []bool{false, true} {
		body := `{"model":"mock","messages":[{"role":"user","content":"hello"}],"stream":false}`
		if stream {
			body = strings.Replace(body, "false", "true", 1)
		}
		v, err := request(s.Client(), s.URL, []byte(body), stream)
		if err != nil {
			t.Fatal(err)
		}
		if v.WorkNS <= 0 || v.WorkFirstNS <= 0 || v.TotalNS < v.WorkNS || v.FirstNS < v.WorkFirstNS {
			t.Fatalf("invalid duration accounting: %+v", v)
		}
		if stream && (v.FirstNS >= v.TotalNS || v.WorkFirstNS >= v.WorkNS) {
			t.Fatalf("SSE was not incremental: %+v", v)
		}
	}
}

func TestResourceSamplingAndMonotonicClock(t *testing.T) {
	if descriptorKind == "unsupported" {
		t.Skip("no platform collector")
	}
	v, err := readProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if v.RSS == 0 || v.Descriptors == 0 || v.CPUSeconds < 0 {
		t.Fatalf("invalid process resources: %+v", v)
	}
	a := nanoNow()
	time.Sleep(time.Millisecond)
	if nanoNow() <= a {
		t.Fatal("measurement clock did not advance")
	}
}
