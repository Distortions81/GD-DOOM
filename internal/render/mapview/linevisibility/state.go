package linevisibility

type Line struct {
	Index int
	BBox  [4]int64
}

type State struct {
	items []int
}

func (s *State) Filter(lines []Line, minX, minY, maxX, maxY int64) []int {
	if s == nil {
		return nil
	}
	s.items = s.items[:0]
	for _, line := range lines {
		if !Intersects(line.BBox, minX, minY, maxX, maxY) {
			continue
		}
		s.items = append(s.items, line.Index)
	}
	return s.items
}

func Intersects(lineBBox [4]int64, minX, minY, maxX, maxY int64) bool {
	lineMaxY := lineBBox[0]
	lineMinY := lineBBox[1]
	lineMaxX := lineBBox[2]
	lineMinX := lineBBox[3]
	if lineMaxX < minX || lineMinX > maxX {
		return false
	}
	if lineMaxY < minY || lineMinY > maxY {
		return false
	}
	return true
}
