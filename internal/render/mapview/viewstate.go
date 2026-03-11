package mapview

import "math"

type SavedView struct {
	CamX   float64
	CamY   float64
	Zoom   float64
	Follow bool
	Valid  bool
}

type ViewState struct {
	CamX       float64
	CamY       float64
	Zoom       float64
	FitZoom    float64
	FollowMode bool
	SavedView  SavedView

	PrevCamX float64
	PrevCamY float64

	RenderCamX float64
	RenderCamY float64
}

type Snapshot struct {
	CamX       float64
	CamY       float64
	Zoom       float64
	FitZoom    float64
	FollowMode bool
	RenderCamX float64
	RenderCamY float64
	PrevCamX   float64
	PrevCamY   float64
}

type Viewport struct {
	Width       int
	Height      int
	RenderAngle uint32
	Rotate      bool
}

type CacheState struct {
	CamX       float64
	CamY       float64
	Zoom       float64
	Angle      uint32
	Rotate     bool
	ViewWidth  int
	ViewHeight int
}

func (s Snapshot) FollowEnabled() bool {
	return s.FollowMode
}

func (s Snapshot) ZoomLevel() float64 {
	return s.Zoom
}

func (s Snapshot) FitZoomLevel() float64 {
	return s.FitZoom
}

func (s Snapshot) Camera() (float64, float64) {
	return s.CamX, s.CamY
}

func (s Snapshot) RenderCamera() (float64, float64) {
	return s.RenderCamX, s.RenderCamY
}

func (s Snapshot) ViewBounds(viewW, viewH int) (left, right, bottom, top float64) {
	zoom := s.Zoom
	left = s.RenderCamX - float64(viewW)/(2*zoom)
	right = s.RenderCamX + float64(viewW)/(2*zoom)
	bottom = s.RenderCamY - float64(viewH)/(2*zoom)
	top = s.RenderCamY + float64(viewH)/(2*zoom)
	return left, right, bottom, top
}

func (s Snapshot) BoundsForViewport(vp Viewport) (left, right, bottom, top float64) {
	return s.ViewBounds(vp.Width, vp.Height)
}

func (s Snapshot) VisibleBounds(vp Viewport, margin float64) (left, right, bottom, top float64) {
	left, right, bottom, top = s.BoundsForViewport(vp)
	if !vp.Rotate {
		left -= margin
		right += margin
		bottom -= margin
		top += margin
		return left, right, bottom, top
	}
	viewHalfW := float64(vp.Width) / (2 * s.Zoom)
	viewHalfH := float64(vp.Height) / (2 * s.Zoom)
	camX, camY := s.RenderCamera()
	r := math.Hypot(viewHalfW, viewHalfH) + margin
	return camX - r, camX + r, camY - r, camY + r
}

func (s Snapshot) CacheState(vp Viewport) CacheState {
	camX, camY := s.RenderCamera()
	return CacheState{
		CamX:       camX,
		CamY:       camY,
		Zoom:       s.Zoom,
		Angle:      vp.RenderAngle,
		Rotate:     vp.Rotate,
		ViewWidth:  vp.Width,
		ViewHeight: vp.Height,
	}
}

func (s *ViewState) Reset(centerX, centerY, worldW, worldH float64, viewW, viewH int, initialZoomMul float64) {
	s.CamX = centerX
	s.CamY = centerY
	s.FitZoom = FitZoomForWorld(worldW, worldH, viewW, viewH)
	s.Zoom = s.FitZoom * initialZoomMul
}

func (s *ViewState) Refit(worldW, worldH float64, viewW, viewH int, initialZoomMul float64) {
	oldFit := s.FitZoom
	s.FitZoom = FitZoomForWorld(worldW, worldH, viewW, viewH)
	if oldFit > 0 {
		s.Zoom = (s.Zoom / oldFit) * s.FitZoom
		return
	}
	s.Zoom = s.FitZoom * initialZoomMul
}

func (s *ViewState) ToggleBigMap(centerX, centerY float64) bool {
	if !s.SavedView.Valid {
		s.SavedView = SavedView{
			CamX:   s.CamX,
			CamY:   s.CamY,
			Zoom:   s.Zoom,
			Follow: s.FollowMode,
			Valid:  true,
		}
		s.FollowMode = false
		s.CamX = centerX
		s.CamY = centerY
		s.Zoom = s.FitZoom
		return true
	}
	s.CamX = s.SavedView.CamX
	s.CamY = s.SavedView.CamY
	s.Zoom = s.SavedView.Zoom
	s.FollowMode = s.SavedView.Follow
	s.SavedView.Valid = false
	return false
}

func (s *ViewState) Snapshot() Snapshot {
	return Snapshot{
		CamX:       s.CamX,
		CamY:       s.CamY,
		Zoom:       s.Zoom,
		FitZoom:    s.FitZoom,
		FollowMode: s.FollowMode,
		RenderCamX: s.RenderCamX,
		RenderCamY: s.RenderCamY,
		PrevCamX:   s.PrevCamX,
		PrevCamY:   s.PrevCamY,
	}
}

func (s *ViewState) Camera() (float64, float64) {
	return s.CamX, s.CamY
}

func (s *ViewState) ToggleFollowMode() bool {
	s.FollowMode = !s.FollowMode
	return s.FollowMode
}

func (s *ViewState) SetFollowMode(v bool) {
	s.FollowMode = v
}

func (s *ViewState) SetZoom(v float64) {
	s.Zoom = v
}

func (s *ViewState) AdjustZoom(zoomStep, wheelY float64) {
	if zoomStep > 0 {
		s.Zoom *= zoomStep
	}
	if zoomStep < 0 {
		s.Zoom /= -zoomStep
	}
	if wheelY > 0 {
		s.Zoom *= 1.1
	}
	if wheelY < 0 {
		s.Zoom /= 1.1
	}
	minZoom := s.FitZoom * 0.05
	maxZoom := s.FitZoom * 200
	if s.Zoom < minZoom {
		s.Zoom = minZoom
	}
	if s.Zoom > maxZoom {
		s.Zoom = maxZoom
	}
}

func (s *ViewState) SetCamera(x, y float64) {
	s.CamX = x
	s.CamY = y
}

func (s *ViewState) Pan(dx, dy float64) {
	s.CamX += dx
	s.CamY += dy
}

func (s *ViewState) CapturePrev() {
	s.PrevCamX = s.CamX
	s.PrevCamY = s.CamY
}

func (s *ViewState) SyncRender() {
	s.CapturePrev()
	s.RenderCamX = s.CamX
	s.RenderCamY = s.CamY
}

func (s *ViewState) PrepareRender(alpha float64) {
	s.RenderCamX = Lerp(s.PrevCamX, s.CamX, alpha)
	s.RenderCamY = Lerp(s.PrevCamY, s.CamY, alpha)
}

func (s *ViewState) WorldToScreen(x, y float64, viewW, viewH int, renderAngle uint32, rotate bool) (float64, float64) {
	dx := x - s.RenderCamX
	dy := y - s.RenderCamY
	if rotate {
		rot := (math.Pi / 2) - angleToRadians(renderAngle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		rdx := dx*cr - dy*sr
		rdy := dx*sr + dy*cr
		dx = rdx
		dy = rdy
	}
	sx := dx*s.Zoom + float64(viewW)/2
	sy := float64(viewH)/2 - dy*s.Zoom
	return sx, sy
}

func (s *ViewState) WorldToScreenViewport(vp Viewport, x, y float64) (float64, float64) {
	return s.WorldToScreen(x, y, vp.Width, vp.Height, vp.RenderAngle, vp.Rotate)
}

func (s *ViewState) ScreenToWorld(sx, sy float64, viewW, viewH int, renderAngle uint32, rotate bool) (float64, float64) {
	dx := (sx - float64(viewW)/2) / s.Zoom
	dy := (float64(viewH)/2 - sy) / s.Zoom
	if rotate {
		rot := (math.Pi / 2) - angleToRadians(renderAngle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		wdx := dx*cr + dy*sr
		wdy := -dx*sr + dy*cr
		dx = wdx
		dy = wdy
	}
	return s.RenderCamX + dx, s.RenderCamY + dy
}

func (s *ViewState) ScreenToWorldViewport(vp Viewport, sx, sy float64) (float64, float64) {
	return s.ScreenToWorld(sx, sy, vp.Width, vp.Height, vp.RenderAngle, vp.Rotate)
}

func FitZoomForWorld(worldW, worldH float64, viewW, viewH int) float64 {
	worldW = math.Max(worldW, 1)
	worldH = math.Max(worldH, 1)
	margin := 0.9
	if viewW < 1 {
		viewW = 1
	}
	if viewH < 1 {
		viewH = 1
	}
	zx := float64(viewW) * margin / worldW
	zy := float64(viewH) * margin / worldH
	return math.Max(math.Min(zx, zy), 0.0001)
}

func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func angleToRadians(a uint32) float64 {
	return float64(a) * 2 * math.Pi / 4294967296.0
}
