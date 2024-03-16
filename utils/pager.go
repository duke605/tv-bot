package utils

import (
	"database/sql"
	"errors"
)

type Pager[T any] interface {
	Next() (T, error)
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

func (p *pager[T]) Next() (T, error) {
	if p.done {
		return *new(T), sql.ErrNoRows
	}

	if p.currIdx+1 > len(p.buf)-1 {
		b, err := p.nextPageFn(p.currPage, p.buf)
		if errors.Is(err, sql.ErrNoRows) {
			p.done = true
			return *new(T), err
		} else if err != nil {
			return *new(T), err
		}

		p.buf = b
		p.currIdx = 0
		p.currPage++
	}

	t := p.buf[p.currIdx]
	p.currIdx++

	return t, nil
}
