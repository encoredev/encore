//go:build !encore_app

package encore

func meta() *AppMetadata {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}

func currentRequest() *Request {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}
