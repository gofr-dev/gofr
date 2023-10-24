package gofr

import (
	"encoding/json"
	"regexp"
	"strings"

	"gofr.dev/pkg/errors"
)

const defaultProjection = "compact"
const fullProjection = "full"

// ProjectionMapType is used to denote the map of resource and it's projections
type ProjectionMapType map[string]string

// regex.MustCompile is a costly operation hence it is better that it is defined globally
var (
	rWordsDotSeparated = regexp.MustCompile(`([a-zA-Z\-]+\.[a-zA-Z]+)`)    //nolint
	rWordsOnly         = regexp.MustCompile(`([a-zA-Z\-]+)`)               //nolint
	rWordsBeforeDot    = regexp.MustCompile(`([a-zA-Z\-]+\.([^a-zA-Z]+))`) //nolint
	rWordsAfterDot     = regexp.MustCompile(`([^a-zA-Z]|^\s*)\.[a-zA-Z]+`) //nolint

	//nolint:gochecknoglobals // Has to be declared global as Struct can't be defined as constants
	invalidProjection = errors.InvalidParam{Param: []string{"projections"}}
)

// Projections takes valid projections as a parameter and returns a map of resource and its projections.
//
//nolint:gocognit,gocyclo // can't reduce complexity further
func (c *Context) Projections(resource string) (ProjectionMapType, error) {
	projectionString := c.req.Param("projections")
	jsonStr := convertProjectionToJSONString(projectionString)

	var projectionRawMap map[string]interface{}

	if err := checkInvalidProjectionString(projectionString); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(jsonStr), &projectionRawMap); err != nil {
		return nil, invalidProjection
	}

	if !validateRawProjectionMap(c, projectionRawMap) {
		return nil, invalidProjection
	}

	// check for validity of resources extracted from the request
	for k := range projectionRawMap {
		if !validateResource(c, k) {
			return nil, invalidProjection
		}
	}

	if _, ok := projectionRawMap[resource]; !ok && len(projectionRawMap) != 0 {
		return nil, invalidProjection
	}

	validShapesMap := getValidShapesMap(c, resource)
	if !validateShapes(validShapesMap) {
		return nil, invalidProjection
	}

	if !validateResource(c, resource) {
		return nil, invalidProjection
	}

	projectionMap, err := getAllDotSeparated(c, projectionString, validShapesMap)
	if err != nil {
		return nil, err
	}

	if len(projectionMap) == 0 {
		projectionMap[resource] = defaultProjection
		return projectionMap, nil
	}

	err = validateProjection("", projectionRawMap, resource)
	if err != nil {
		return nil, err
	}

	createProjections("", projectionRawMap, projectionMap)

	return projectionMap, nil
}

func checkShapeInResource(shapeList []string, shape string) bool {
	for _, s := range shapeList {
		if s == shape {
			return true
		}
	}

	return false
}

func getValidShapesMap(c *Context, resource string) map[string]bool {
	validShapesMap := make(map[string]bool)

	for _, v := range c.ResourceCustomShapes[resource] {
		validShapesMap[strings.ToLower(v)] = true
	}

	if _, ok := validShapesMap[defaultProjection]; !ok {
		validShapesMap[defaultProjection] = true
	}

	return validShapesMap
}

// validateShapes checks the valid shapes following two simple rules.
// 1. If the size of shapes is 1 then only compact should be present.
// 2. If the size of shapes is more than 1, compact and full both should be present.
func validateShapes(shapes map[string]bool) bool {
	_, compactOk := shapes["compact"]
	_, fullOk := shapes["full"]

	if len(shapes) == 1 && !compactOk {
		return false
	}

	if len(shapes) > 1 && (!compactOk || !fullOk) {
		return false
	}

	return true
}

// makeAllSubresource is called whenever the user wants to put full shape for all the sub resources.
func makeAllSubresource(c *Context, projectionMap ProjectionMapType, resource string) {
	for i := range c.Gofr.ResourceMap[resource] {
		currentResource := c.Gofr.ResourceMap[resource][i]
		if checkShapeInResource(c.Gofr.ResourceCustomShapes[currentResource], fullProjection) {
			projectionMap[currentResource] = fullProjection
		} else {
			projectionMap[currentResource] = defaultProjection
		}

		if _, ok := c.Gofr.ResourceMap[currentResource]; ok {
			makeAllSubresource(c, projectionMap, currentResource)
		}
	}
}

// getAllDotSeparated extracts all the projections which are directly specified
// in the projection string separated by dot.
func getAllDotSeparated(c *Context, projectionStr string, validShapesMap map[string]bool) (ProjectionMapType, error) {
	projectionMap := make(ProjectionMapType)
	matchingWords := rWordsDotSeparated.FindAllString(projectionStr, -1)

	for i := range matchingWords {
		splitStr := strings.Split(matchingWords[i], ".")

		if _, ok := validShapesMap[splitStr[1]]; !ok {
			return nil, invalidProjection
		}

		if _, ok := projectionMap[splitStr[0]]; ok {
			return nil, invalidProjection
		}

		if splitStr[1] == fullProjection {
			makeAllSubresource(c, projectionMap, splitStr[0])
		}

		projectionMap[splitStr[0]] = splitStr[1]
	}

	return projectionMap, nil
}

func validateResource(c *Context, resource string) bool {
	for key, val := range c.ResourceMap {
		if key == resource {
			return true
		}

		for v := range val {
			if val[v] == resource {
				return true
			}
		}
	}

	return false
}

// convertProjectionToJSONString converts a projection string to a json string following simple logic
func convertProjectionToJSONString(projection string) string {
	// converting all [ to :{
	remainingProjection := strings.ReplaceAll(projection, "[", ":{")

	// converting all ] to }
	remainingProjection = strings.ReplaceAll(remainingProjection, "]", "}")

	// converting . to :
	remainingProjection = strings.ReplaceAll(remainingProjection, ".", ":")

	projectionStr := rWordsOnly.ReplaceAllStringFunc(remainingProjection, func(str string) string {
		return "\"" + str + "\""
	})

	return "{" + projectionStr + "}"
}

// checkInMap checks whether a value exist in a map. The map to look for is
// selected based on checkShape variable
func checkInMap(key string, inputMap interface{}) error {
	if _, ok := inputMap.(map[string]interface{})[key]; ok {
		return invalidProjection
	}

	return nil
}

// validateProjection validates raw projection parsed from the request
//
//nolint:gocognit // cognitive complexity cannot be decreased further.
func validateProjection(parentKey string, projectionRawMap map[string]interface{}, resource string) error {
	err := checkInMap(parentKey, projectionRawMap)
	if err != nil {
		return err
	}

	for key, val := range projectionRawMap {
		// if the projection includes the resource targeted by the endpoint,
		// it MUST NOT be wrapped by the resource name and square brackets. i.e
		// that resource should always be at the zeroth level of the raw projection map
		if key == resource && parentKey != "" && parentKey != resource {
			return invalidProjection
		}

		switch v := val.(type) {
		case map[string]interface{}:
			// recursion if the val is a map again
			err = validateProjection(key, v, resource)
		default:
			// no need validate as this block will be reached only when the val is a string
			// and that has been already validated by getAllDotSeparated function
			continue
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// createProjections creates resultant projection map from raw projections parsed from the request
//
//nolint:gocognit // splitting the code will reduce readability
func createProjections(parentKey string, projectionRawMap map[string]interface{}, projectionMap ProjectionMapType) {
	for key, val := range projectionRawMap {
		var projection string

		switch v := val.(type) {
		case map[string]interface{}:
			createProjections(key, v, projectionMap)

			if parentKey == "" {
				projection = defaultProjection
			}
		default:
			projection = val.(string)
		}

		if _, ok := projectionMap[key]; !ok && projection != "" {
			projectionMap[key] = projection
		}
	}
}

func checkInvalidProjectionString(projectionString string) error {
	onlyResources := rWordsBeforeDot.FindAllString(projectionString, -1)

	projStrLen := len(projectionString)
	if projStrLen == 0 {
		return nil
	}

	if len(onlyResources) > 0 {
		return invalidProjection
	}

	if projectionString[projStrLen-1] == '.' {
		// This special case has to be introduced because for some reason projections=resource. is not
		// being detected in the above condition
		return invalidProjection
	}

	onlyShapes := rWordsAfterDot.FindAllString(projectionString, -1)
	if len(onlyShapes) > 0 {
		return invalidProjection
	}

	return nil
}

//nolint:gocognit // Can't reduce complexity further. introducing another function will increase stack
func validateRawProjectionMap(c *Context, inputMap map[string]interface{}) bool {
	for k, v := range inputMap {
		switch val := v.(type) {
		case map[string]interface{}:
			for child := range val {
				if !checkInList(child, c.ResourceMap[k]) {
					return false
				}
			}

			if !validateRawProjectionMap(c, val) {
				return false
			}
		case string:
			for _, v2 := range c.ResourceMap[k] {
				if _, ok := inputMap[v2]; ok {
					return false
				}
			}
		}
	}

	return true
}

func checkInList(element string, list []string) bool {
	for _, v := range list {
		if v == element {
			return true
		}
	}

	return false
}
