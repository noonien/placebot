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

type SpiralDraw struct {
	Position []int
	Bitmap   [][]byte

	width  int
	height int
}

func NewSpiralDraw(pos []int, data string) *SpiralDraw {
	bitmap := parseBitmap(data)

	width := 0
	for _, row := range bitmap {
		if len(row) > width {
			width = len(row)
		}
	}
	height := len(bitmap)

	sd := &SpiralDraw{
		Position: pos,
		Bitmap:   bitmap,
		width:    width,
		height:   height,
	}
	return sd
}

func (sd *SpiralDraw) Next() *Tile {
	cX, cY := sd.width/2, sd.height/2

	x, y := 0, 0
	dx, dy := 0, -1

	max := sd.width
	if sd.height > max {
		max = sd.height
	}

	for i := 0; i <= max*max; i++ {
		posX, posY := cX+x-1, cY+y-1
		if 0 <= posX && posX < sd.width && 0 <= posY && posY < sd.height {
			newTile := sd.checkPos(posX, posY)
			if newTile != nil {
				return newTile
			}
		}

		if x == y || (x < 0 && x == -y) || (x > 0 && x == 1-y) {
			dx, dy = -dy, dx
		}

		x, y = x+dx, y+dy
	}

	return nil
}

func (sd *SpiralDraw) checkPos(x, y int) *Tile {
	if y > sd.height {
		return nil
	}

	row := sd.Bitmap[y]
	if x > len(row) {
		return nil
	}

	col := row[x]
	if col == 255 {
		return nil
	}

	absX, absY := sd.Position[0]+x, sd.Position[1]+y
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
