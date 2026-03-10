package marks

type Mark struct {
	ID int
	X  float64
	Y  float64
}

type State struct {
	items  []Mark
	nextID int
	limit  int
}

func New(limit int) State {
	if limit <= 0 {
		limit = 10
	}
	return State{
		items:  make([]Mark, 0, limit),
		nextID: 1,
		limit:  limit,
	}
}

func (s *State) Add(x, y float64) (id int, ok bool) {
	if len(s.items) >= s.limit {
		return 0, false
	}
	id = s.nextID
	s.items = append(s.items, Mark{ID: id, X: x, Y: y})
	s.nextID++
	return id, true
}

func (s *State) Clear() {
	s.items = s.items[:0]
}

func (s *State) Count() int {
	return len(s.items)
}

func (s *State) Items() []Mark {
	return s.items
}
