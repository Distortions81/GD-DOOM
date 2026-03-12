package doomruntime

import "gddoom/internal/sessionflow"

func frontendNextSelectableOptionRow(cur, dir int) int {
	return sessionflow.NextSelectableOptionRow(frontendOptionsSelectableRows[:], cur, dir)
}

func frontendNextMouseSensitivity(speed float64, dir int) float64 {
	return sessionflow.NextMouseSensitivity(speed, dir)
}

func frontendMouseSensitivityDot(speed float64) int {
	return sessionflow.MouseSensitivityDot(speed)
}

func clampFrontendMouseLookSpeed(v float64) float64 {
	return sessionflow.ClampMouseLookSpeed(v)
}

func frontendMouseSensitivitySpeedForDot(dot int) float64 {
	return sessionflow.MouseSensitivitySpeedForDot(dot)
}

func frontendVolumeDot(v float64) int {
	return sessionflow.VolumeDot(v)
}

func frontendMessagesPatch(enabled bool) string {
	return sessionflow.MessagesPatch(enabled)
}

func frontendDetailPatch(low bool) string {
	return sessionflow.DetailPatch(low)
}
