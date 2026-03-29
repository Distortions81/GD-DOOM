package doomruntime

import (
	"fmt"
	"math"
)

const doomGammaLevels = 5

const defaultGammaLevel = 2

const gammaDisplayGamma = 2.2

// Level 0 is true 1:1 gamma at 2.2. The remaining four steps cover a darker
// 2.8 base and then brighter corrections up to 1.4.
var gammaTargetLevels = [...]float64{2.2, 2.8, 2.4, 2.0, 1.4}

var doomGammaTables = buildGammaTables()

func buildGammaTables() [doomGammaLevels][256]uint8 {
	var tables [doomGammaLevels][256]uint8
	for level := 0; level < doomGammaLevels; level++ {
		ratio := gammaTargetLevels[level] / gammaDisplayGamma
		for i := 0; i < 256; i++ {
			if i == 255 {
				tables[level][i] = 255
				continue
			}
			v := math.Pow(float64(i)/255.0, ratio) * 255.0
			iv := int(math.Round(v))
			if iv < 0 {
				iv = 0
			}
			if iv > 255 {
				iv = 255
			}
			tables[level][i] = uint8(iv)
		}
	}
	return tables
}

func gammaMessage(level int) string {
	level = clampGamma(level)
	if level == 0 {
		return fmt.Sprintf("Gamma OFF [%.1f]", gammaTargetLevels[level])
	}
	return fmt.Sprintf("Gamma %d [%.1f]", level, gammaTargetLevels[level])
}
