package main

import "math/rand"

type FillGenerator interface {
	Reset(width, height int)
	Next() []int
}

type SpiralFill struct {
	width  int
	height int

	cX, cY int
	x, y   int
	dx, dy int

	iters int
}

func (f *SpiralFill) Reset(width, height int) {
	iters := width
	if height > iters {
		iters = height
	}

	*f = SpiralFill{
		width:  width,
		height: height,

		cX: width / 2,
		cY: height / 2,

		x: 0, y: 0,
		dx: 0, dy: 0,

		iters: iters,
	}
}

func (f *SpiralFill) Next() []int {
	// restore state
	x, y, dx, dy := f.x, f.y, f.dx, f.dy

	for i := 0; i <= f.iters; i++ {
		posX, posY := f.cX+x-1, f.cY+y-1

		if x == y || (x < 0 && x == -y) || (x > 0 && x == 1-y) {
			dx, dy = -dy, dx
		}

		x, y = x+dx, y+dy

		if 0 <= posX && posX < f.width && 0 <= posY && posY < f.height {
			// save state
			f.x, f.y, f.dx, f.dy = x, y, dx, dy
			f.iters -= i
			return []int{posX, posY}
		}

	}

	f.iters = -1
	return nil
}

type RandomFill struct {
	width  int
	height int

	x, a, c, m int

	iters int
}

func (f *RandomFill) Reset(width, height int) {
	length := width * height

	m := 8
	for m < length {
		m <<= 1
	}

	c := m/6 + rand.Intn(5*m/6)
	c |= 1

	a := m / 12 * rand.Intn(m/6)
	a = 4*a + 1

	x := rand.Intn(m)

	*f = RandomFill{
		width:  width,
		height: height,

		x: x, a: a, c: a, m: m,

		iters: length,
	}
}

func (f *RandomFill) Next() []int {
	if f.iters <= 0 {
		return nil
	}

	length := f.width * f.height
	x, a, c, m := f.x, f.a, f.c, f.m

	for {
		x = (a*x + c) % m
		if x < length {
			break
		}
	}

	f.x = x
	f.iters -= 1
	return []int{x % f.width, x / f.height}
}

type RowFill struct {
	Inverted bool

	width  int
	height int

	i int
}

func (f *RowFill) Reset(width, height int) {
	*f = RowFill{
		Inverted: f.Inverted,

		width:  width,
		height: height,
	}
}

func (f *RowFill) Next() []int {
	width, height, i := f.width, f.height, f.i
	total := width * height

	if i >= total {
		return nil
	}

	if f.Inverted {
		i = total - 1 - i
	}

	f.i += 1
	return []int{i % width, i / height}
}
