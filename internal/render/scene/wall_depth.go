package scene

type WallDepthColumn struct {
	DepthQ uint16
	Top    int
	Bottom int
	Closed bool
}

func WallDepthColumnOccludesPoint(col WallDepthColumn, y int, depthQ uint16) bool {
	if col.DepthQ == 0xFFFF || depthQ <= col.DepthQ {
		return false
	}
	if col.Closed {
		return true
	}
	if col.Top <= col.Bottom && y >= col.Top && y <= col.Bottom {
		return true
	}
	return false
}

func WallDepthColumnOccludesBBox(col WallDepthColumn, y0, y1 int, depthQ uint16) bool {
	if col.DepthQ == 0xFFFF || depthQ <= col.DepthQ {
		return false
	}
	if col.Closed {
		return true
	}
	if col.Top <= col.Bottom && y0 >= col.Top && y1 <= col.Bottom {
		return true
	}
	return false
}

func WallDepthColumnHasAnyOccluder(col WallDepthColumn, y0, y1 int, depthQ uint16) bool {
	if col.DepthQ == 0xFFFF || depthQ <= col.DepthQ {
		return false
	}
	if col.Closed {
		return true
	}
	if col.Top <= col.Bottom && y0 <= col.Bottom && y1 >= col.Top {
		return true
	}
	return false
}
