package websocket

type Options func(u *WSUpgrader)

//
// func WithHandshakeTimeout(t time.Duration) Options {
//	return func(u *WSUpgrader) {
//		u.HandshakeTimeout = t
//	}
//}
//
// func WithReadBufferSize(size int) Options {
//	return func(u *WSUpgrader) {
//		u.ReadBufferSize = size
//	}
//}
//
// func WithWriteBufferSize(size int) Options {
//	return func(u *WSUpgrader) {
//		u.WriteBufferSize = size
//	}
//}
//
// func WithSubprotocols(subprotocols ...string) Options {
//	return func(u *wsUpgrader) {
//		u.Subprotocols = subprotocols
//	}
//}
//
// func WithError(fn func(w http.ResponseWriter, r *http.Request, status int, reason error)) Options {
//	return func(u *wsUpgrader) {
//		u.Error = fn
//	}
//}
//
// func WithCheckOrigin(fn func(r *http.Request) bool) Options {
//	return func(u *wsUpgrader) {
//		u.CheckOrigin = fn
//	}
//}
//
// func WithCompression() Options {
//	return func(u *wsUpgrader) {
//		u.EnableCompression = true
//	}
//}
