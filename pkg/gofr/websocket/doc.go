// Package websocket provides WebSocket support for the GoFr framework.
//
// It offers a configurable [Upgrader], a thread-safe [Connection] wrapper
// with JSON and text binding, and a [Manager] that tracks active WebSocket
// connections by ID for lifecycle management including upgrade, list,
// retrieve, and close operations.
package websocket
