package doomruntime

import "gddoom/internal/sessiontransition"

const meltVirtualH = sessiontransition.MeltVirtualH

func initMeltColumns(width int) []int {
	return sessiontransition.InitMeltColumns(width)
}

func initMeltColumnsScaled(width int, mult int) []int {
	return sessiontransition.InitMeltColumnsScaled(width, mult)
}

func stepMeltColumns(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int) bool {
	return sessiontransition.StepMeltColumns(y, width, height, fromPix, toPix, workPix, ticks)
}

func stepMeltSlicesVirtual(y []int, virtualH int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	return sessiontransition.StepMeltSlicesVirtual(y, virtualH, width, height, fromPix, toPix, workPix, ticks, slices)
}

func sourcePortMeltRNGScale(height int) int {
	scale := height / meltVirtualH
	if scale < 1 {
		return 1
	}
	return scale
}
