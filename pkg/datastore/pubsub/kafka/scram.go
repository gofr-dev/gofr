package kafka

import (
	"crypto/sha512"

	"github.com/xdg/scram"
)

//nolint:gochecknoglobals // only SHA512 should be used for scram authentication
var (
	SHA512 scram.HashGeneratorFcn = sha512.New
)

type XDGSCRAMClient struct {
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

/*
		Begin prepares the client for the SCRAM exchange with
	    the server with a user name and a password
*/
func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	client, err := x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}

	x.ClientConversation = client.NewConversation()

	return nil
}

/*
Step steps client through the SCRAM exchange. It is
called repeatedly until it errors or `Done` returns true.
*/
func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)

	return
}

/*
Done should return true when the SCRAM conversation
is over.
*/
func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
