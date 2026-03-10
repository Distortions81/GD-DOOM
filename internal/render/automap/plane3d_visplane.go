package automap

import "gddoom/internal/render/scene"

const plane3DUnset int16 = -1

type plane3DVisplane struct {
	key    plane3DKey
	minX   int
	maxX   int
	top    []int16
	bottom []int16
}

type plane3DAllocator func(key plane3DKey, start, stop, viewW int) *plane3DVisplane

func newPlane3DVisplane(key plane3DKey, start, stop, viewW int) *plane3DVisplane {
	pl := &plane3DVisplane{
		key:    key,
		minX:   start,
		maxX:   stop,
		top:    make([]int16, viewW+2),
		bottom: make([]int16, viewW+2),
	}
	for i := range pl.top {
		pl.top[i] = plane3DUnset
		pl.bottom[i] = plane3DUnset
	}
	return pl
}

func ensurePlane3DForRange(planes map[plane3DKey][]*plane3DVisplane, key plane3DKey, start, stop, viewW int) (*plane3DVisplane, bool) {
	return ensurePlane3DForRangeAlloc(planes, key, start, stop, viewW, newPlane3DVisplane)
}

func ensurePlane3DForRangeAlloc(planes map[plane3DKey][]*plane3DVisplane, key plane3DKey, start, stop, viewW int, alloc plane3DAllocator) (*plane3DVisplane, bool) {
	scenePlanes := make(map[scene.PlaneKey][]*scene.PlaneVisplane, len(planes))
	backref := make(map[*scene.PlaneVisplane]*plane3DVisplane)
	for k, list := range planes {
		sk := plane3DKeyToScene(k)
		sceneList := make([]*scene.PlaneVisplane, 0, len(list))
		for _, pl := range list {
			sp := &scene.PlaneVisplane{
				Key:    sk,
				MinX:   pl.minX,
				MaxX:   pl.maxX,
				Top:    append([]int16(nil), pl.top...),
				Bottom: append([]int16(nil), pl.bottom...),
			}
			sceneList = append(sceneList, sp)
			backref[sp] = pl
		}
		scenePlanes[sk] = sceneList
	}
	var wrapped scene.PlaneAllocator
	if alloc != nil {
		wrapped = func(key scene.PlaneKey, start, stop, viewW int) *scene.PlaneVisplane {
			local := alloc(plane3DKey{
				height: key.Height, light: key.Light, flat: key.Flat,
				fallback: key.Fallback, sky: key.Sky, floor: key.Floor,
			}, start, stop, viewW)
			sp := &scene.PlaneVisplane{
				Key:    key,
				MinX:   local.minX,
				MaxX:   local.maxX,
				Top:    local.top,
				Bottom: local.bottom,
			}
			backref[sp] = local
			return sp
		}
	}
	sp, created := scene.EnsurePlaneForRangeAlloc(scenePlanes, plane3DKeyToScene(key), start, stop, viewW, wrapped)
	for sk, list := range scenePlanes {
		localKey := plane3DKey{height: sk.Height, light: sk.Light, flat: sk.Flat, fallback: sk.Fallback, sky: sk.Sky, floor: sk.Floor}
		localList := make([]*plane3DVisplane, 0, len(list))
		for _, item := range list {
			local := backref[item]
			if local == nil {
				local = &plane3DVisplane{
					key:    localKey,
					minX:   item.MinX,
					maxX:   item.MaxX,
					top:    item.Top,
					bottom: item.Bottom,
				}
			} else {
				local.minX = item.MinX
				local.maxX = item.MaxX
				local.top = item.Top
				local.bottom = item.Bottom
			}
			localList = append(localList, local)
		}
		planes[localKey] = localList
	}
	if sp == nil {
		return nil, created
	}
	return backref[sp], created
}

func markPlane3DColumnRange(pl *plane3DVisplane, x, top, bottom int, ceilingclip, floorclip []int) bool {
	if pl == nil {
		return false
	}
	sp := &scene.PlaneVisplane{Key: plane3DKeyToScene(pl.key), MinX: pl.minX, MaxX: pl.maxX, Top: pl.top, Bottom: pl.bottom}
	ok := scene.MarkPlaneColumnRange(sp, x, top, bottom, ceilingclip, floorclip)
	pl.minX = sp.MinX
	pl.maxX = sp.MaxX
	pl.top = sp.Top
	pl.bottom = sp.Bottom
	return ok
}

func makePlane3DSpans(pl *plane3DVisplane, viewH int, out []plane3DSpan) []plane3DSpan {
	if pl == nil {
		return out
	}
	sceneOut := make([]scene.PlaneSpan, 0, len(out))
	sp := &scene.PlaneVisplane{Key: plane3DKeyToScene(pl.key), MinX: pl.minX, MaxX: pl.maxX, Top: pl.top, Bottom: pl.bottom}
	sceneOut = scene.MakePlaneSpans(sp, viewH, sceneOut)
	out = out[:0]
	for _, s := range sceneOut {
		out = append(out, plane3DSpan{
			y:  s.Y,
			x1: s.X1,
			x2: s.X2,
			key: plane3DKey{
				height: s.Key.Height, light: s.Key.Light, flat: s.Key.Flat,
				fallback: s.Key.Fallback, sky: s.Key.Sky, floor: s.Key.Floor,
			},
		})
	}
	return out
}
