package gofr

import (
	"fmt"
	"strconv"
	"strings"
)

// parseSchedule parses the schedule string and creates a job struct with filled times to launch,
// returning an error if the syntax is incorrect.
func parseSchedule(s string) (*job, error) {
	var err error

	j := &job{}
	s = MatchSpaces.ReplaceAllLiteralString(s, " ")
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

	// Merge day and dayOfWeek fields
	mergeDays(j)

	return j, nil
}

// mergeDays clears the day or dayOfWeek map depending on which one is fully populated.
func mergeDays(j *job) {
	switch {
	case len(j.day) < 31 && len(j.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		j.dayOfWeek = make(map[int]struct{})
	case len(j.dayOfWeek) < 7 && len(j.day) == 31: // dayOfWeek set, but not day, clear day
		j.day = make(map[int]struct{})
	}
}

// parsePart parses an individual schedule part from the schedule string.
func parsePart(s string, minValue, maxValue int) (map[int]struct{}, error) {
	// Wildcard pattern
	if s == "*" {
		return getDefaultJobField(minValue, maxValue, 1), nil
	}

	// */2, 1-59/5 pattern
	if matches := MatchN.FindStringSubmatch(s); matches != nil {
		return parseSteps(s, matches[1], matches[2], minValue, maxValue)
	}

	// 1,2,4 or 1,2,10-15,20,30-45 pattern
	return parseRange(s, minValue, maxValue)
}

// parseSteps handles patterns like */2, 1-59/5.
func parseSteps(s, match1, match2 string, minValue, maxValue int) (map[int]struct{}, error) {
	localMin := minValue
	localMax := maxValue

	if match1 != "" && match1 != "*" {
		rng := MatchRange.FindStringSubmatch(match1)
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

// parseRange handles patterns like 1,2,4 or 1,2,10-15,20,30-45.
func parseRange(s string, minValue, maxValue int) (map[int]struct{}, error) {
	r := make(map[int]struct{})
	parts := strings.Split(s, ",")

	for _, part := range parts {
		if err := processPart(part, minValue, maxValue, r); err != nil {
			return nil, err
		}
	}

	if len(r) == 0 {
		return nil, errParsing{invalidPart: s}
	}

	return r, nil
}

// processPart determines whether to handle a part as a value or a range.
func processPart(part string, minValue, maxValue int, r map[int]struct{}) error {
	if rng := MatchRange.FindStringSubmatch(part); rng != nil {
		return handleRange(rng, minValue, maxValue, r)
	}
	return handleValue(part, minValue, maxValue, r)
}

// handleValue parses a single value and checks if it falls within the acceptable range.
func handleValue(value string, minValue, maxValue int, r map[int]struct{}) error {
	i, err := strconv.Atoi(value)
	if err != nil {
		return errParsing{invalidPart: value}
	}

	if i < minValue || i > maxValue {
		return errOutOfRange{
			rangeVal: i,
			input:    value,
			min:      minValue,
			max:      maxValue,
		}
	}

	r[i] = struct{}{}
	return nil
}

// handleRange parses a range (e.g., 1-5) and populates the map with the range values.
func handleRange(rng []string, minValue, maxValue int, r map[int]struct{}) error {
	localMin, errMin := strconv.Atoi(rng[1])
	localMax, errMax := strconv.Atoi(rng[2])

	if errMin != nil || errMax != nil {
		return fmt.Errorf("invalid range format: %v", rng)
	}

	if localMin > localMax {
		return errOutOfRange{
			rangeVal: fmt.Sprintf("%d-%d", localMin, localMax),
			input:    fmt.Sprintf("%d-%d", localMin, localMax),
			min:      minValue,
			max:      maxValue,
		}
	}

	if localMin < minValue || localMax > maxValue {
		return errOutOfRange{
			rangeVal: fmt.Sprintf("%d-%d", localMin, localMax),
			input:    fmt.Sprintf("%d-%d", localMin, localMax),
			min:      minValue,
			max:      maxValue,
		}
	}

	for i := localMin; i <= localMax; i++ {
		r[i] = struct{}{}
	}
	return nil
}

// getDefaultJobField creates a default map based on the provided increment, min, and max values.
func getDefaultJobField(minValue, maxValue, incr int) map[int]struct{} {
	r := make(map[int]struct{})

	for i := minValue; i <= maxValue; i += incr {
		r[i] = struct{}{}
	}

	return r
}
