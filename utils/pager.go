package utils

type Pager[T any] interface {
	Next() (T, bool, error)
}

type pager[T any] struct {
	buf        []T
	currPage   int
	currIdx    int
	done       bool
	nextPageFn func(currPage int, buf []T) ([]T, error)
}

func NewPager[T any](nextFn func(currPage int, buf []T) ([]T, error)) Pager[T] {
	return &pager[T]{
		nextPageFn: nextFn,
		buf:        []T{},
	}
}

func (p *pager[T]) Next() (T, bool, error) {
	if p.done {
		return *new(T), false, nil
	}

	if p.currIdx > len(p.buf)-1 {
		b, err := p.nextPageFn(p.currPage, p.buf)
		if err != nil {
			return *new(T), false, err
		} else if len(b) == 0 {
			p.done = true
			return *new(T), false, err
		}

		p.buf = b
		p.currIdx = 0
		p.currPage++
	}

	t := p.buf[p.currIdx]
	p.currIdx++

	return t, true, nil
}
