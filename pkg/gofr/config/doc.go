// Package config defines the configuration interface used throughout the
// GoFr framework.
//
// It provides a minimal key-value abstraction with [Config.Get] and
// [Config.GetOrDefault] methods, allowing application components to retrieve
// configuration values without coupling to a specific source such as
// environment variables or files.
package config
