//go:build goexperiment.jsonv2

package jsonenc

// enabled is false when encoding/json is built on its v2 implementation (GOEXPERIMENT=jsonv2),
// whose output this package has not been checked against; every call then falls back.
const enabled = false
