package utils

func MapSlice[T, R any](slice []T, mapper func(T, int) R) []R {
	ret := make([]R, len(slice))
	for i, t := range slice {
		ret[i] = mapper(t, i)
	}

	return ret
}

func Map[K comparable, V, R any](m map[K]V, mapper func(V, K) R) []R {
	ret := make([]R, 0, len(m))
	for k, v := range m {
		ret = append(ret, mapper(v, k))
	}

	return ret
}

func Clamp[T any](t []T, limit int) []T {
	if len(t) < limit {
		return t
	}

	return t[:limit:limit]
}
