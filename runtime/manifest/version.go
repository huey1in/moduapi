package manifest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

type version struct {
	n   [3]uint64
	pre string
}

func parseVersion(s string) (version, error) {
	m := semverPattern.FindStringSubmatch(s)
	var v version
	if m == nil {
		return v, fmt.Errorf("invalid semantic version %q", s)
	}
	for i := range 3 {
		n, e := strconv.ParseUint(m[i+1], 10, 64)
		if e != nil {
			return v, e
		}
		v.n[i] = n
	}
	v.pre = m[4]
	for _, p := range strings.Split(v.pre, ".") {
		if numeric(p) && len(p) > 1 && p[0] == '0' {
			return v, fmt.Errorf("numeric prerelease has leading zero")
		}
	}
	return v, nil
}
func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func compare(a, b version) int {
	for i := range 3 {
		if a.n[i] < b.n[i] {
			return -1
		}
		if a.n[i] > b.n[i] {
			return 1
		}
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	aa, bb := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < min(len(aa), len(bb)); i++ {
		x, y := aa[i], bb[i]
		if x == y {
			continue
		}
		xn, yn := numeric(x), numeric(y)
		if xn && yn {
			if len(x) < len(y) {
				return -1
			}
			if len(x) > len(y) {
				return 1
			}
		} else if xn != yn {
			if xn {
				return -1
			}
			return 1
		}
		if x < y {
			return -1
		}
		return 1
	}
	if len(aa) < len(bb) {
		return -1
	}
	return 1
}

// Satisfies accepts an explicit, space-separated conjunction of comparators.
// No wildcards, implicit coercion, OR ranges, or npm-style shorthand.
func Satisfies(current, constraint string) (bool, error) {
	v, e := parseVersion(current)
	if e != nil {
		return false, e
	}
	parts := strings.Fields(constraint)
	if len(parts) == 0 {
		return false, fmt.Errorf("empty version requirement")
	}
	ok := true
	for _, part := range parts {
		op := "="
		raw := part
		for _, prefix := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(part, prefix) {
				op = prefix
				raw = strings.TrimPrefix(part, prefix)
				break
			}
		}
		target, e := parseVersion(raw)
		if e != nil {
			return false, e
		}
		c := compare(v, target)
		switch op {
		case "=":
			ok = ok && c == 0
		case ">=":
			ok = ok && c >= 0
		case "<=":
			ok = ok && c <= 0
		case ">":
			ok = ok && c > 0
		case "<":
			ok = ok && c < 0
		}
	}
	return ok, nil
}
