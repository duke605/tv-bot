package utils

func Map[T, R any](slice []T, mapper func(T, int) R) []R {
	ret := make([]R, len(slice))
	for i, t := range slice {
		ret[i] = mapper(t, i)
	}

	return ret
}
