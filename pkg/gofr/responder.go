package gofr

type Responder interface {
	Respond(data interface{}, err error)
}
