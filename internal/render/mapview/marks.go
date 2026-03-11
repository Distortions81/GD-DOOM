package mapview

type Mark struct {
	ID int
	X  float64
	Y  float64
}

type MarksState struct {
	items  []Mark
	nextID int
	limit  int
}

func NewMarksState(limit int) MarksState {
	if limit <= 0 {
		limit = 10
	}
	return MarksState{
		items:  make([]Mark, 0, limit),
		nextID: 1,
		limit:  limit,
	}
}

func (s *MarksState) Add(x, y float64) (id int, ok bool) {
	if len(s.items) >= s.limit {
		return 0, false
	}
	id = s.nextID
	s.items = append(s.items, Mark{ID: id, X: x, Y: y})
	s.nextID++
	return id, true
}

func (s *MarksState) Clear() {
	s.items = s.items[:0]
}

func (s *MarksState) Count() int {
	return len(s.items)
}

func (s *MarksState) Items() []Mark {
	return s.items
}
