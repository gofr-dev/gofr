package types

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/errors"
)

// TimeInterval denotes a time interval in string and provides functionality to validate it
type TimeInterval string

const noOfDaysInWeek = 7

// this will compile the regex once instead of compiling it each time when it is being called.
var durationReg = regexp.MustCompile(`P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// Check validates the TimeInterval value.
// It ensures that the time interval follows the ISO 8601 standard format and contains valid timestamps.
// The format should be either "start_time/PnYnMnDTnHnMnS" or "start_time/end_time".
// If any validation fails, it returns an InvalidParam error with the corresponding parameter name.
func (t TimeInterval) Check() error {
	// Time intervals MUST follow the templates above from the ISO 8601 standard.
	timeIntervalArray := strings.Split(string(t), "/")

	const timeIntervalArrayLen = 2

	if len(timeIntervalArray) < timeIntervalArrayLen {
		return errors.InvalidParam{Param: []string{"timeInterval"}}
	}

	_, err := time.Parse(time.RFC3339, timeIntervalArray[0])
	if err != nil {
		return errors.InvalidParam{Param: []string{"startTime"}}
	}

	// start time and duration
	// YYYY-MM-DDTHH:MM:SSZ/PnYnMnDTnHnMnS
	err = Validate(Duration(timeIntervalArray[1]))
	if err != nil {
		// start and end time
		// YYYY-MM-DDTHH:MM:SSZ/YYYY-MM-DDTHH:MM:SSZ
		_, err = time.Parse(time.RFC3339, timeIntervalArray[1])
		if err != nil {
			return errors.InvalidParam{Param: []string{"endTime"}}
		}
	}

	return nil
}

// TimeInterval MUST follow the templates from the ISO 8601 standard.

// GetStartAndEndTime will return startTime and endTime for a given TimeInterval
// In case of an error, will return zero value of time.Time along with error
func (t TimeInterval) GetStartAndEndTime() (startTime, endTime time.Time, err error) {
	// Check the format of TimeInterval
	if e := t.Check(); e != nil {
		return time.Time{}, time.Time{}, e
	}

	timeIntervalArray := strings.Split(string(t), "/")

	// not checking any error since all the validation for a TimeInterval is done by t.Check()
	startTime, _ = time.Parse(time.RFC3339, timeIntervalArray[0])

	if endTime, err = time.Parse(time.RFC3339, timeIntervalArray[1]); err == nil {
		return startTime, endTime, nil
	}

	// duration: PnYnMnDTnHnMnS
	return startTime, getEndTime(startTime, timeIntervalArray[1]), nil
}

// getEndTime returns the endTime for the given startTime and duration.
func getEndTime(startTime time.Time, duration string) time.Time {
	var years, months, weeks, days, hours, minutes, seconds int

	dur := durationReg.FindStringSubmatch(duration)
	years, _ = strconv.Atoi(dur[1])
	months, _ = strconv.Atoi(dur[2])
	weeks, _ = strconv.Atoi(dur[3])

	days += noOfDaysInWeek * weeks
	d, _ := strconv.Atoi(dur[4])
	days += d

	hours, _ = strconv.Atoi(dur[5])
	minutes, _ = strconv.Atoi(dur[6])
	seconds, _ = strconv.Atoi(dur[7])

	endTime := startTime.AddDate(years, months, days)
	nanoSecondCount := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	endTime = endTime.Add(nanoSecondCount)

	return endTime
}
