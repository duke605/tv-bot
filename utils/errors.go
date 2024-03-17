package utils

func ReturnPanic(ptr *error) {
	if perr, ok := recover().(error); ok {
		*ptr = perr
	}
}

func Must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
