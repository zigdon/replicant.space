package errors

type PostError struct {
	Err error
	Status int
	Body []byte
}

func (pe PostError) Error() string {
	return pe.Err.Error()
}

