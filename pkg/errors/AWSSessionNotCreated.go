// Package errors have the predefined errors that can be used to denote various types of errors
package errors

// AWSSessionNotCreated error constant for issues in creating AWS Session
const AWSSessionNotCreated = Error("some issue while creation of AWS session, session not initialized")
