package responder

type Responder interface {
	Respond(data interface{}, err error)
}
