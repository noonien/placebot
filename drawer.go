package main

import (
	"strings"
)

type Drawer interface {
	Next() *Tile
}

type MultiDrawer []Drawer

func (m MultiDrawer) Next() *Tile {
	for _, d := range m {
		tile := d.Next()
		if tile != nil {
			return tile
		}
	}

	return nil
}

type BitmapDraw struct {
	Position []int
	Fill     FillGenerator
	Bitmap   [][]byte

	width  int
	height int
}

func NewBitmapDraw(pos []int, fill FillGenerator, data string) *BitmapDraw {
	bitmap := parseBitmap(data)

	width := 0
	for _, row := range bitmap {
		if len(row) > width {
			width = len(row)
		}
	}
	height := len(bitmap)

	return &BitmapDraw{
		Position: pos,
		Fill:     fill,
		Bitmap:   bitmap,
		width:    width,
		height:   height,
	}
}

func (b *BitmapDraw) Next() *Tile {
	b.Fill.Reset(b.width, b.height)

	for {
		pos := b.Fill.Next()
		if pos == nil {
			break
		}

		tile := b.checkPos(pos[0], pos[1])
		if tile != nil {
			return tile
		}
	}

	return nil
}

func (b *BitmapDraw) checkPos(x, y int) *Tile {
	if y > b.height {
		return nil
	}

	row := b.Bitmap[y]
	if x > len(row) {
		return nil
	}

	col := row[x]
	if col == 255 {
		return nil
	}

	absX, absY := b.Position[0]+x, b.Position[1]+y
	if absX > 1000 || absY > 1000 {
		return nil
	}

	curCol := Bitmap[absY][absX]
	if col == curCol {
		return nil
	}

	return &Tile{
		X:     absX,
		Y:     absY,
		Color: col,
	}
}

func parseBitmap(data string) [][]byte {
	lines := strings.Fields(strings.ToLower(data))

	bitmap := make([][]byte, len(lines))
	for i, line := range lines {
		row := make([]byte, len(line))
		bitmap[i] = row

		for i, r := range line {
			if '0' <= r && r <= '9' {
				row[i] = byte(r - '0')
			} else if 'a' <= r && r <= 'f' {
				row[i] = byte(r - 'a' + 10)
			} else {
				row[i] = 255
			}
		}
	}

	return bitmap
}
