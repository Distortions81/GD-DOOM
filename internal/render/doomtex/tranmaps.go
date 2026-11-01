package doomtex

import (
	"crypto/sha256"
	"encoding/binary"
	"gddoom/internal/wad"
	"strings"
	"sync"
)

// TRANMAP and the named tables referenced by 260 sidedefs use the global WAD
// namespace. Invalid-sized names retain their ordinary texture interpretation.
func LoadBoomTranMaps(w *wad.File) map[string][]byte {
	names := map[string]bool{"TRANMAP": true}
	var lines []byte
	for _, l := range w.Lumps {
		if l.Name == "LINEDEFS" {
			lines, _ = w.LumpDataView(l)
			continue
		}
		if l.Name != "SIDEDEFS" || len(lines) == 0 {
			continue
		}
		sides, err := w.LumpDataView(l)
		if err != nil {
			lines = nil
			continue
		}
		for i := 0; i+14 <= len(lines); i += 14 {
			if binary.LittleEndian.Uint16(lines[i+6:i+8]) != 260 {
				continue
			}
			side := int(binary.LittleEndian.Uint16(lines[i+10 : i+12]))
			offset := side*30 + 20
			if side == 65535 || offset+8 > len(sides) {
				continue
			}
			names[strings.ToUpper(strings.TrimRight(string(sides[offset:offset+8]), "\x00"))] = true
		}
		lines = nil
	}

	out := make(map[string][]byte)
	for name := range names {
		if l, ok := w.LumpByName(name); ok && l.Size == 65536 {
			if data, err := w.LumpDataView(l); err == nil {
				out[name] = data
			}
		}
	}
	return out
}

var boomTranMapCache sync.Map

// GenerateBoomTranMap matches Boom/DSDA's 66% source, 34% destination filter.
// Ties select the highest palette index, and comparison stays in 12-bit fixed
// arithmetic rather than rounding an RGB average before the nearest-color test.
func GenerateBoomTranMap(palette []byte) []byte {
	if len(palette) < 1024 {
		return nil
	}
	key := sha256.Sum256(palette[:1024])
	if v, ok := boomTranMapCache.Load(key); ok {
		return v.([]byte)
	}
	const sourceWeight = 66 * 4096 / 100
	const destWeight = 4096 - sourceWeight
	var rgb [256][3]int64
	var total [256]int64
	for i := range rgb {
		for c := 0; c < 3; c++ {
			v := int64(palette[i*4+c])
			rgb[i][c] = v
			total[i] += v * v
		}
		total[i] <<= 11
	}
	out := make([]byte, 65536)
	for dst := 0; dst < 256; dst++ {
		for src := 0; src < 256; src++ {
			r := rgb[src][0]*sourceWeight + rgb[dst][0]*destWeight
			g := rgb[src][1]*sourceWeight + rgb[dst][1]*destWeight
			b := rgb[src][2]*sourceWeight + rgb[dst][2]*destWeight
			best := int64(1<<63 - 1)
			for color := 255; color >= 0; color-- {
				err := total[color] - rgb[color][0]*r - rgb[color][1]*g - rgb[color][2]*b
				if err < best {
					best = err
					out[dst*256+src] = byte(color)
				}
			}
		}
	}
	actual, _ := boomTranMapCache.LoadOrStore(key, out)
	return actual.([]byte)
}
