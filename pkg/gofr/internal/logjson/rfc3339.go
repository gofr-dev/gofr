package logjson

import (
	"sync/atomic"
	"time"
)

// second is the RFC 3339 rendering of one second in one location: the date and time up to the
// seconds, and the zone suffix. A log line's times all fall in the current second, so almost every
// call reuses it and only appends the fraction.
type second struct {
	unix int64
	loc  *time.Location
	date [len("2006-01-02T15:04:05")]byte
	zone string // "Z" or "+05:30"
}

// lastSecond is the most recently rendered second. Entries are never changed once stored, and a
// newer second replaces an older one, so concurrent loggers share one rendering per second.
//
//nolint:gochecknoglobals // a rendering of the wall clock, the same for every logger.
var lastSecond atomic.Pointer[second]

// fracDigits is the nanosecond digits of RFC3339Nano's ".999999999".
const fracDigits = 9

// AppendRFC3339Nano appends exactly what t.AppendFormat(dst, time.RFC3339Nano) appends.
func AppendRFC3339Nano(dst []byte, t time.Time) []byte {
	unix, loc := t.Unix(), t.Location()

	s := lastSecond.Load()
	if s == nil || s.unix != unix || s.loc != loc {
		if s = renderSecond(t, unix, loc, s); s == nil {
			return t.AppendFormat(dst, time.RFC3339Nano)
		}
	}

	dst = append(dst, s.date[:]...)

	if ns := t.Nanosecond(); ns != 0 {
		dst = appendFraction(dst, ns)
	}

	return append(dst, s.zone...)
}

// renderSecond renders t's second and stores it as lastSecond, or returns nil when it should not
// replace cur: an older second, or another location in the same second, takes the plain format
// path instead, so a process logging in two locations does not allocate on every line. The year
// range is the one RFC3339's fixed-width layout covers (year 0 is left out because Truncate rounds
// relative to year 1).
func renderSecond(t time.Time, unix int64, loc *time.Location, cur *second) *second {
	if cur != nil && cur.unix >= unix {
		return nil
	}

	if y := t.Year(); y < 1 || y > maxYear {
		return nil
	}

	var buf [len(time.RFC3339) + 1]byte

	s := &second{unix: unix, loc: loc}

	b := t.Truncate(time.Second).AppendFormat(buf[:0], time.RFC3339)
	if len(b) <= len(s.date) {
		return nil // not reachable for years 1-9999, kept so a layout change cannot misrender
	}

	copy(s.date[:], b)
	s.zone = string(b[len(s.date):])

	lastSecond.CompareAndSwap(cur, s)

	return s
}

// appendFraction appends ".ddddddddd" for ns nanoseconds with trailing zeros cut, as the ".999999999"
// of RFC3339Nano does. ns is not zero.
func appendFraction(dst []byte, ns int) []byte {
	var b [1 + fracDigits]byte

	b[0] = '.'

	for i := fracDigits; i >= 1; i-- {
		b[i] = byte('0' + ns%decimal)
		ns /= decimal
	}

	n := len(b)
	for b[n-1] == '0' {
		n--
	}

	return append(dst, b[:n]...)
}
