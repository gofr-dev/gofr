package service

type OAuthErr struct {
	Err     error
	Message string
}

func (o OAuthErr) Error() string {
	if o.Message == "" && o.Err == nil {
		return "unknown error"
	} else if o.Message == "" {
		return o.Err.Error()
	} else if o.Err == nil {
		return o.Message
	}

	return o.Message + "\n" + o.Err.Error()
}
