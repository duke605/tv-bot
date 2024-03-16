package utils

type Batcher[T any] interface {
	Add(T) error
	Flush() error
}

type batcher[T any] struct {
	limit   int
	buf     []T
	flushFn func([]T) error
}

func NewBatcher[T any](limit int, flushFn func([]T) error) Batcher[T] {
	return &batcher[T]{
		limit:   limit,
		flushFn: flushFn,
		buf:     make([]T, 0, limit),
	}
}

func (bat *batcher[T]) Add(t T) error {
	bat.buf = append(bat.buf, t)
	if len(bat.buf) >= bat.limit {
		return bat.Flush()
	}

	return nil
}

func (bat *batcher[T]) Flush() error {
	if len(bat.buf) == 0 {
		return nil
	}

	if err := bat.flushFn(bat.buf); err != nil {
		return err
	}

	bat.buf = bat.buf[:0]
	return nil
}

type DualBatcher[T, V any] interface {
	Add(T, V) error
	Flush() error
}

type dualBatcher[T, V any] struct {
	limit   int
	bufT    []T
	bufV    []V
	flushFn func([]T, []V) error
}

func NewDualBatcher[T, V any](limit int, flushFn func([]T, []V) error) DualBatcher[T, V] {
	return &dualBatcher[T, V]{
		limit:   limit,
		flushFn: flushFn,
		bufT:    make([]T, 0, limit),
		bufV:    make([]V, 0, limit),
	}
}

func (bat *dualBatcher[T, V]) Add(t T, v V) error {
	bat.bufT = append(bat.bufT, t)
	bat.bufV = append(bat.bufV, v)
	if len(bat.bufT) >= bat.limit {
		return bat.Flush()
	}

	return nil
}

func (bat *dualBatcher[T, V]) Flush() error {
	if len(bat.bufT) == 0 {
		return nil
	}

	if err := bat.flushFn(bat.bufT, bat.bufV); err != nil {
		return err
	}

	bat.bufT = bat.bufT[:0]
	bat.bufV = bat.bufV[:0]
	return nil
}
