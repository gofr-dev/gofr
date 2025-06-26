package gofr

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// this will compile the regex once instead of compiling it each time when it is being called.
var (
	matchSpaces = regexp.MustCompile(`\s+`)
	matchN      = regexp.MustCompile(`(.*)/(\d+)`)
	matchRange  = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

// parseSchedule parses schedule string and create job struct with filled times to launch,
// or error if syntax is wrong.
func parseSchedule(s string) (*job, error) {
	var err error

	j := &job{}
	s = matchSpaces.ReplaceAllLiteralString(s, " ")
	s = strings.Trim(s, " ")
	parts := strings.Split(s, " ")

	var partsItr int

	switch len(parts) {
	case schedulePartsWithSecond:
		j.sec, err = parsePart(parts[partsItr], 0, seconds)
		if err != nil {
			return nil, err
		}

		partsItr++
	case scheduleParts:
		partsItr = 0
	default:
		return nil, errBadScheduleFormat
	}

	j.min, err = parsePart(parts[partsItr], 0, minutes)
	if err != nil {
		return nil, err
	}

	j.hour, err = parsePart(parts[partsItr+1], 0, hrs)
	if err != nil {
		return nil, err
	}

	j.day, err = parsePart(parts[partsItr+2], 1, days)
	if err != nil {
		return nil, err
	}

	j.month, err = parsePart(parts[partsItr+3], 1, months)
	if err != nil {
		return nil, err
	}

	j.dayOfWeek, err = parsePart(parts[partsItr+4], 0, dayOfWeek)
	if err != nil {
		return nil, err
	}

	//  day/dayOfWeek combination
	mergeDays(j)

	return j, nil
}

func mergeDays(j *job) {
	switch {
	case len(j.day) < 31 && len(j.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		j.dayOfWeek = make(map[int]struct{})
	case len(j.dayOfWeek) < 7 && len(j.day) == 31: // dayOfWeek set, but not day, clear day
		j.day = make(map[int]struct{})
	}
}

// parsePart parse individual schedule part from schedule string.
func parsePart(s string, minValue, maxValue int) (map[int]struct{}, error) {
	// wildcard pattern
	if s == "*" {
		return getDefaultJobField(minValue, maxValue, 1), nil
	}

	// */2 1-59/5 pattern
	if matches := matchN.FindStringSubmatch(s); matches != nil {
		return parseSteps(s, matches[1], matches[2], minValue, maxValue)
	}

	// 1,2,4 or 1,2,10-15,20,30-45 pattern
	return parseRange(s, minValue, maxValue)
}

func parseSteps(s, match1, match2 string, minValue, maxValue int) (map[int]struct{}, error) {
	localMin := minValue
	localMax := maxValue

	if match1 != "" && match1 != "*" {
		rng := matchRange.FindStringSubmatch(match1)
		if rng == nil {
			return nil, errParsing{match1, s}
		}

		localMin, _ = strconv.Atoi(rng[1])
		localMax, _ = strconv.Atoi(rng[2])

		if localMin < minValue || localMax > maxValue {
			return nil, errOutOfRange{rng[1], s, minValue, maxValue}
		}
	}

	n, _ := strconv.Atoi(match2)

	return getDefaultJobField(localMin, localMax, n), nil
}

func parseRange(s string, minValue, maxValue int) (map[int]struct{}, error) {
	r := make(map[int]struct{})
	parts := strings.Split(s, ",")

	for _, part := range parts {
		if err := parseSingleOrRange(part, minValue, maxValue, r); err != nil {
			return nil, err
		}
	}

	if len(r) == 0 {
		return nil, errParsing{invalidPart: s}
	}

	return r, nil
}

func parseSingleOrRange(part string, minValue, maxValue int, r map[int]struct{}) error {
	if rng := matchRange.FindStringSubmatch(part); rng != nil {
		localMin, _ := strconv.Atoi(rng[1])
		localMax, _ := strconv.Atoi(rng[2])

		if localMin < minValue || localMax > maxValue {
			return errOutOfRange{part, part, minValue, maxValue}
		}

		for i := localMin; i <= localMax; i++ {
			r[i] = struct{}{}
		}
	} else {
		i, err := strconv.Atoi(part)
		if err != nil {
			return errParsing{part, part}
		}

		if i < minValue || i > maxValue {
			return errOutOfRange{part, part, minValue, maxValue}
		}

		r[i] = struct{}{}
	}

	return nil
}

func getDefaultJobField(minValue, maxValue, incr int) map[int]struct{} {
	r := make(map[int]struct{})

	for i := minValue; i <= maxValue; i += incr {
		r[i] = struct{}{}
	}

	return r
}

func getTick(t time.Time) *tick {
	return &tick{
		sec:       t.Second(),
		min:       t.Minute(),
		hour:      t.Hour(),
		day:       t.Day(),
		month:     int(t.Month()),
		dayOfWeek: int(t.Weekday()),
	}
}

func (j *job) tick(t *tick) bool {
	if _, ok := j.min[t.min]; !ok {
		return false
	}

	if _, ok := j.hour[t.hour]; !ok {
		return false
	}

	// cumulative day and dayOfWeek, as it should be
	_, day := j.day[t.day]
	_, dayOfWeek := j.dayOfWeek[t.dayOfWeek]

	if !day && !dayOfWeek {
		return false
	}

	if _, ok := j.month[t.month]; !ok {
		return false
	}

	if j.sec != nil {
		if _, ok := j.sec[t.sec]; !ok {
			return false
		}
	} else {
		if t.sec != 0 {
			return false
		}
	}

	return true
}
