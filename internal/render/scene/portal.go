package scene

type WallPortalInput struct {
	FrontFloor float64
	FrontCeil  float64
	BackFloor  float64
	BackCeil   float64
	EyeZ       float64

	FrontFloorFlat   string
	FrontCeilingFlat string
	BackFloorFlat    string
	BackCeilingFlat  string

	FrontLight int16
	BackLight  int16

	BackExists         bool
	DoomSectorLighting bool
	IsFrontCeilingSky  bool
	IsBackCeilingSky   bool
}

type WallPortalState struct {
	WorldTop    float64
	WorldBottom float64
	WorldHigh   float64
	WorldLow    float64
	TopWall     bool
	BottomWall  bool
	MarkCeiling bool
	MarkFloor   bool
	SolidWall   bool
}

func ClassifyWallPortal(in WallPortalInput) WallPortalState {
	s := WallPortalState{
		WorldTop:    in.FrontCeil - in.EyeZ,
		WorldBottom: in.FrontFloor - in.EyeZ,
		MarkCeiling: true,
		MarkFloor:   true,
		SolidWall:   !in.BackExists,
	}
	s.WorldHigh = s.WorldTop
	s.WorldLow = s.WorldBottom

	if in.BackExists {
		s.WorldHigh = in.BackCeil - in.EyeZ
		s.WorldLow = in.BackFloor - in.EyeZ
		skyPortal := in.IsFrontCeilingSky && in.IsBackCeilingSky
		if skyPortal {
			s.WorldTop = s.WorldHigh
		}
		lightDiff := in.BackLight != in.FrontLight && in.DoomSectorLighting
		s.MarkFloor = s.WorldLow != s.WorldBottom ||
			in.BackFloorFlat != in.FrontFloorFlat ||
			lightDiff
		s.MarkCeiling = s.WorldHigh != s.WorldTop ||
			in.BackCeilingFlat != in.FrontCeilingFlat ||
			lightDiff
		if skyPortal && in.BackCeil != in.FrontCeil {
			s.MarkCeiling = true
		}
		if in.BackCeil <= in.FrontFloor || in.BackFloor >= in.FrontCeil {
			s.MarkFloor = true
			s.MarkCeiling = true
			s.SolidWall = true
		}
		s.TopWall = s.WorldHigh < s.WorldTop
		s.BottomWall = s.WorldLow > s.WorldBottom
	}

	if in.FrontFloor >= in.EyeZ {
		s.MarkFloor = false
	}
	if in.FrontCeil <= in.EyeZ && !in.IsFrontCeilingSky {
		s.MarkCeiling = false
	}
	return s
}
