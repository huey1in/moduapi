package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if e := json.Unmarshal(b, &s); e != nil {
		return e
	}
	v, e := time.ParseDuration(s)
	*d = Duration(v)
	return e
}

type Config struct {
	Health          HealthConfig `json:"health"`
	Sticky          StickyConfig `json:"sticky"`
	LeaseTTL        Duration     `json:"lease_ttl"`
	CleanupInterval Duration     `json:"cleanup_interval"`
	StateTTL        Duration     `json:"state_ttl"`
	MaxCandidates   int          `json:"max_candidates"`
	MaxLeases       int          `json:"max_leases"`
}
type HealthConfig struct {
	Enabled          bool     `json:"enabled"`
	Alpha            float64  `json:"ewma_alpha"`
	FailureThreshold int      `json:"failure_threshold"`
	Cooldown         Duration `json:"cooldown"`
	MinWeightFactor  float64  `json:"min_weight_factor"`
}
type StickyConfig struct {
	Mode            string   `json:"mode"`
	TTL             Duration `json:"ttl"`
	MaxEntries      int      `json:"max_entries"`
	EscapeUnhealthy bool     `json:"escape_on_unhealthy"`
	EscapeCapacity  bool     `json:"escape_on_capacity"`
}

func Defaults() Config {
	return Config{
		Health:   HealthConfig{true, .2, 3, Duration(5 * time.Second), .1},
		Sticky:   StickyConfig{"prefer", Duration(time.Hour), 10000, true, true},
		LeaseTTL: Duration(6 * time.Minute), CleanupInterval: Duration(time.Second), StateTTL: Duration(time.Hour), MaxCandidates: 10000, MaxLeases: 100000,
	}
}
func ParseConfig(raw []byte) (Config, error) {
	c := Defaults()
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return c, fmt.Errorf("configuration must be object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&c); e != nil {
		return c, e
	}
	if d.Decode(new(any)) != io.EOF {
		return c, fmt.Errorf("trailing JSON")
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.Health.Alpha <= 0 || c.Health.Alpha > 1 || c.Health.MinWeightFactor <= 0 || c.Health.MinWeightFactor > 1 || c.Health.FailureThreshold < 1 || c.Health.FailureThreshold > 1000 || c.Health.Cooldown <= 0 || c.Health.Cooldown > Duration(time.Hour) {
		return fmt.Errorf("invalid health configuration")
	}
	if c.Sticky.Mode != "off" && c.Sticky.Mode != "prefer" && c.Sticky.Mode != "strict" {
		return fmt.Errorf("invalid sticky mode")
	}
	if c.Sticky.TTL <= 0 || c.Sticky.TTL > Duration(24*time.Hour) || c.Sticky.MaxEntries < 1 || c.Sticky.MaxEntries > 1000000 {
		return fmt.Errorf("invalid sticky bounds")
	}
	if c.LeaseTTL < Duration(time.Millisecond) || c.LeaseTTL > Duration(24*time.Hour) || c.CleanupInterval < Duration(time.Millisecond) || c.CleanupInterval > Duration(30*time.Second) || c.StateTTL < c.LeaseTTL || c.StateTTL > Duration(7*24*time.Hour) || c.MaxCandidates < 1 || c.MaxCandidates > 100000 || c.MaxLeases < 1 || c.MaxLeases > 1000000 {
		return fmt.Errorf("invalid state/lease bounds")
	}
	return nil
}
