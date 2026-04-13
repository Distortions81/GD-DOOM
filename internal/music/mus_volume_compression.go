package music

type MUSVolumeCompressionStats struct {
	Ratio float64

	NoteOnCount          int
	AvgNoteVelocityBefore float64
	AvgNoteVelocityAfter  float64

	ControllerVolumeCount   int
	AvgControllerVolumeBefore float64
	AvgControllerVolumeAfter  float64

	ControllerExpressionCount   int
	AvgControllerExpressionBefore float64
	AvgControllerExpressionAfter  float64
}

func ApplyMUSVolumeCompression(parsed *ParsedMUS, ratio float64) *ParsedMUS {
	if parsed == nil {
		return nil
	}
	ratio = clampMUSVolumeCompression(ratio)
	if ratio <= 1 {
		return parsed
	}
	out := make([]Event, len(parsed.events))
	copy(out, parsed.events)
	for i := range out {
		switch out[i].Type {
		case EventNoteOn:
			if out[i].B != 0 {
				out[i].B = compressMUSLevel(out[i].B, ratio)
			}
		case EventControlChange:
			if out[i].A == controllerVol || out[i].A == controllerExpr {
				out[i].B = compressMUSLevel(out[i].B, ratio)
			}
		}
	}
	return &ParsedMUS{
		events:            out,
		estimatedPCMBytes: parsed.estimatedPCMBytes,
	}
}

func AnalyzeMUSVolumeCompression(parsed *ParsedMUS, ratio float64) MUSVolumeCompressionStats {
	stats := MUSVolumeCompressionStats{Ratio: clampMUSVolumeCompression(ratio)}
	if parsed == nil {
		return stats
	}

	var noteBefore, noteAfter int
	var volBefore, volAfter int
	var exprBefore, exprAfter int

	for _, ev := range parsed.events {
		switch ev.Type {
		case EventNoteOn:
			if ev.B == 0 {
				continue
			}
			stats.NoteOnCount++
			noteBefore += int(ev.B)
			noteAfter += int(compressMUSLevel(ev.B, stats.Ratio))
		case EventControlChange:
			switch ev.A {
			case controllerVol:
				stats.ControllerVolumeCount++
				volBefore += int(ev.B)
				volAfter += int(compressMUSLevel(ev.B, stats.Ratio))
			case controllerExpr:
				stats.ControllerExpressionCount++
				exprBefore += int(ev.B)
				exprAfter += int(compressMUSLevel(ev.B, stats.Ratio))
			}
		}
	}

	if stats.NoteOnCount > 0 {
		stats.AvgNoteVelocityBefore = float64(noteBefore) / float64(stats.NoteOnCount)
		stats.AvgNoteVelocityAfter = float64(noteAfter) / float64(stats.NoteOnCount)
	}
	if stats.ControllerVolumeCount > 0 {
		stats.AvgControllerVolumeBefore = float64(volBefore) / float64(stats.ControllerVolumeCount)
		stats.AvgControllerVolumeAfter = float64(volAfter) / float64(stats.ControllerVolumeCount)
	}
	if stats.ControllerExpressionCount > 0 {
		stats.AvgControllerExpressionBefore = float64(exprBefore) / float64(stats.ControllerExpressionCount)
		stats.AvgControllerExpressionAfter = float64(exprAfter) / float64(stats.ControllerExpressionCount)
	}

	return stats
}
