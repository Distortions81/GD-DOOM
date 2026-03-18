package scene

import (
	"math"
	"strconv"
	"strings"

	"gddoom/internal/mapdata"
)

type Texture struct {
	RGBA   []byte
	Width  int
	Height int
}

type Projection struct {
	DrawW   int
	DrawH   int
	SampleW int
	SampleH int
}

func TextureCandidates(mapName mapdata.MapName, normalizeName func(string) string) []string {
	name := strings.ToUpper(strings.TrimSpace(string(mapName)))
	out := make([]string, 0, 5)
	add := func(c string) {
		if normalizeName != nil {
			c = normalizeName(c)
		}
		if c == "" {
			return
		}
		for _, ex := range out {
			if ex == c {
				return
			}
		}
		out = append(out, c)
	}
	if len(name) == 4 && name[0] == 'E' && name[2] == 'M' && name[1] >= '0' && name[1] <= '9' {
		switch int(name[1] - '0') {
		case 1:
			add("SKY1")
		case 2:
			add("SKY2")
		case 3:
			add("SKY3")
		case 4:
			add("SKY4")
		}
	}
	if strings.HasPrefix(name, "MAP") && len(name) >= 5 {
		if n, err := strconv.Atoi(name[3:]); err == nil {
			switch {
			case n >= 1 && n <= 11:
				add("SKY1")
			case n >= 12 && n <= 20:
				add("SKY2")
			case n >= 21:
				add("SKY3")
			}
		}
	}
	add("SKY1")
	add("SKY2")
	add("SKY3")
	add("SKY4")
	return out
}

func TextureEntryForMap(mapName mapdata.MapName, wallTexBank map[string]Texture, normalizeName func(string) string) (string, Texture, bool) {
	for _, name := range TextureCandidates(mapName, normalizeName) {
		key := name
		if normalizeName != nil {
			key = normalizeName(name)
		}
		tex, ok := wallTexBank[key]
		if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			continue
		}
		return key, tex, true
	}
	return "", Texture{}, false
}

func TextureForMap(mapName mapdata.MapName, wallTexBank map[string]Texture, normalizeName func(string) string) (Texture, bool) {
	_, tex, ok := TextureEntryForMap(mapName, wallTexBank, normalizeName)
	return tex, ok
}

func SampleUV(screenX, screenY, viewW, viewH int, focal, camAngle float64, texW, texH int) (u, v int) {
	if texW <= 0 || texH <= 0 {
		return 0, 0
	}
	if focal <= 1e-6 {
		focal = 1
	}
	angle := SampleAngle(screenX, viewW, focal, camAngle)
	uScale := float64(texW*4) / (2 * math.Pi)
	u = wrapIndex(int(math.Floor(angle*uScale)), texW)

	cy := float64(viewH) * 0.5
	if cy <= 1e-6 {
		return u, 0
	}
	yn := (float64(screenY) + 0.5) / cy
	if yn < 0 {
		yn = 0
	}
	if yn > 1 {
		yn = 1
	}
	v = int(math.Floor(yn * float64(texH-1)))
	if v < 0 {
		v = 0
	}
	if v >= texH {
		v = texH - 1
	}
	return u, v
}

func ProjectedSampleIndex(drawCoord, drawSize, sampleSize int) int {
	if drawSize <= 0 || sampleSize <= 0 {
		return 0
	}
	sample := int((float64(drawCoord) + 0.5) * float64(sampleSize) / float64(drawSize))
	if sample < 0 {
		return 0
	}
	if sample >= sampleSize {
		return sampleSize - 1
	}
	return sample
}

func ProjectedSampleUV(drawX, drawY, drawW, drawH, sampleW, sampleH int, focal, camAngle float64, texW, texH int) (u, v int) {
	if drawW <= 0 || drawH <= 0 || sampleW <= 0 || sampleH <= 0 || texW <= 0 || texH <= 0 {
		return 0, 0
	}
	sampleX := ProjectedSampleIndex(drawX, drawW, sampleW)
	sampleY := ProjectedSampleIndex(drawY, drawH, sampleH)
	angle := SampleAngle(sampleX, sampleW, focal, camAngle)
	uScale := float64(texW*4) / (2 * math.Pi)
	u = wrapIndex(int(math.Floor(angle*uScale)), texW)
	iscale := DoomSkyIScale(sampleW)
	frac := 100.0 + ((float64(sampleY) - float64(sampleH)*0.5) * iscale)
	v = wrapIndex(int(math.Floor(frac)), texH)
	return u, v
}

func SampleAngle(screenX, viewW int, focal, camAngle float64) float64 {
	if focal <= 1e-6 {
		focal = 1
	}
	cx := float64(viewW) * 0.5
	sampleX := float64(screenX) + 0.5
	return camAngle + math.Atan((cx-sampleX)/focal)
}

func EffectiveTextureHeight(tex Texture) int {
	if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return 1
	}
	for y := tex.Height - 1; y >= 0; y-- {
		rowStart := y * tex.Width * 4
		opaque := false
		for x := 0; x < tex.Width; x++ {
			if tex.RGBA[rowStart+x*4+3] != 0 {
				opaque = true
				break
			}
		}
		if opaque {
			return y + 1
		}
	}
	return 1
}

func ProjectionSize(viewW, viewH, outputW, outputH int, sourcePort bool) Projection {
	drawW := max(viewW, 1)
	drawH := max(viewH, 1)
	sampleW := drawW
	sampleH := drawH
	if sourcePort {
		if outputW > 0 {
			sampleW = outputW
		}
		if outputH > 0 {
			sampleH = outputH
		}
		sampleW = max(sampleW, 1)
		sampleH = max(sampleH, 1)
	}
	return Projection{DrawW: drawW, DrawH: drawH, SampleW: sampleW, SampleH: sampleH}
}

func BuildLookup(drawW, drawH, sampleW, sampleH int, focal, camAngle float64, texW, texH int) ([]int, []int) {
	if drawW <= 0 || drawH <= 0 || sampleW <= 0 || sampleH <= 0 || texW <= 0 || texH <= 0 {
		return nil, nil
	}
	col := make([]int, drawW)
	row := make([]int, drawH)
	uScale := float64(texW*4) / (2 * math.Pi)
	cx := float64(sampleW) * 0.5
	for x := 0; x < drawW; x++ {
		sampleX := ProjectedSampleIndex(x, drawW, sampleW)
		angle := camAngle + math.Atan((cx-(float64(sampleX)+0.5))/focal)
		col[x] = wrapIndex(int(math.Floor(angle*uScale)), texW)
	}
	iscale := DoomSkyIScale(sampleW)
	for y := 0; y < drawH; y++ {
		sampleY := ProjectedSampleIndex(y, drawH, sampleH)
		frac := 100.0 + ((float64(sampleY) - float64(sampleH)*0.5) * iscale)
		row[y] = wrapIndex(int(math.Floor(frac)), texH)
	}
	return col, row
}

func DoomSkyIScale(viewW int) float64 {
	if viewW <= 0 {
		return 1
	}
	return 320.0 / float64(viewW)
}

func wrapIndex(v, mod int) int {
	if mod <= 0 {
		return 0
	}
	v %= mod
	if v < 0 {
		v += mod
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
