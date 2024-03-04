package pubsub

type Health struct {
	Writer map[string]string
	Reader map[string]map[string]string
}
