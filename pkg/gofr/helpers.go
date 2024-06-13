package gofr

import (
	"strings"
)

func SplitEnv(envString, splitString string) []string {
	if envString == "" {
		return []string{}
	}

	splitArray := strings.Split(envString, splitString)

	tempArray := []string{}

	for _, ele := range splitArray {
		if ele == "" {
			continue
		}
		tempArray = append(tempArray, ele)
	}
	return tempArray
}
