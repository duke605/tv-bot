package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPagerRunsThroughAllElements(t *testing.T) {
	// Arranging
	elements := []int{}
	expElements := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	chunk := 2
	pager := NewPager(func(currPage int, buf []int) ([]int, error) {
		start := currPage * chunk
		end := start + chunk

		buf = buf[:0]
		for i := start; i < len(expElements) && i < end; i++ {
			buf = append(buf, expElements[i])
		}

		return buf, nil
	})

	// Acting
	for {
		i, more, _ := pager.Next()
		if !more {
			break
		}

		elements = append(elements, i)
	}

	// Asserting
	assert.Equal(t, expElements, elements)
}
