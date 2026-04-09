package doomruntime

func flipSpriteOpaqueRectX(rect spriteOpaqueRect, texW int) spriteOpaqueRect {
	if texW <= 0 {
		return rect
	}
	minX := texW - 1 - rect.maxX()
	maxX := texW - 1 - rect.minX()
	return packSpriteOpaqueRect(minX, maxX, rect.minY(), rect.maxY())
}

func (g *game) clearBillboardPlaneOccluderRows() {
	if g == nil {
		return
	}
	for y := range g.billboardPlaneOccluderRows {
		g.billboardPlaneOccluderRows[y] = g.billboardPlaneOccluderRows[y][:0]
	}
}

func (g *game) ensureBillboardPlaneOccluderRows() [][]billboardPlaneOccluderSpan {
	if g == nil || g.viewH <= 0 {
		return nil
	}
	if len(g.billboardPlaneOccluderRows) != g.viewH {
		g.billboardPlaneOccluderRows = make([][]billboardPlaneOccluderSpan, g.viewH)
	}
	g.clearBillboardPlaneOccluderRows()
	return g.billboardPlaneOccluderRows
}

func clipRangeAgainstBillboardPlaneOccluders(l, r int, depthQ uint16, occluders []billboardPlaneOccluderSpan, out []solidSpan) []solidSpan {
	out = out[:0]
	if r < l {
		return out
	}
	cur := l
	for _, occ := range occluders {
		if depthQ <= occ.DepthQ {
			continue
		}
		if occ.R < cur {
			continue
		}
		if occ.L > r {
			break
		}
		if occ.L > cur {
			right := occ.L - 1
			if right > r {
				right = r
			}
			if right >= cur {
				out = append(out, solidSpan{L: cur, R: right})
			}
		}
		if occ.R+1 > cur {
			cur = occ.R + 1
		}
		if cur > r {
			return out
		}
	}
	if cur <= r {
		out = append(out, solidSpan{L: cur, R: r})
	}
	return out
}

func (g *game) appendBillboardPlaneOccluderRow(y, l, r int, depthQ uint16) {
	if g == nil || y < 0 || y >= len(g.billboardPlaneOccluderRows) {
		return
	}
	if l < 0 {
		l = 0
	}
	if r >= g.viewW {
		r = g.viewW - 1
	}
	if l > r {
		return
	}
	row := g.billboardPlaneOccluderRows[y]
	next := billboardPlaneOccluderSpan{
		L:      l,
		R:      r,
		DepthQ: depthQ,
	}
	idx := len(row)
	for idx > 0 {
		prev := row[idx-1]
		if prev.L < next.L || (prev.L == next.L && (prev.R < next.R || (prev.R == next.R && prev.DepthQ <= next.DepthQ))) {
			break
		}
		idx--
	}
	row = append(row, billboardPlaneOccluderSpan{})
	copy(row[idx+1:], row[idx:])
	row[idx] = next
	g.billboardPlaneOccluderRows[y] = row
}

func (g *game) appendBillboardOpaqueRectPlaneOccluders(rects []spriteOpaqueRect, texW int, flip bool, dstX, dstY, scale float64, clipTop, clipBottom int, depthQ uint16, clipSpans []solidSpan) {
	if g == nil || len(rects) == 0 || scale <= 0 || g.viewW <= 0 || g.viewH <= 0 {
		return
	}
	for _, rect := range rects {
		if flip {
			rect = flipSpriteOpaqueRectX(rect, texW)
		}
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, scale, clipTop, clipBottom, g.viewW, g.viewH)
		if !ok {
			continue
		}
		if g.billboardClippingEnabled() && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		if !g.billboardClippingEnabled() {
			if len(clipSpans) == 0 {
				for y := y0; y <= y1; y++ {
					g.appendBillboardPlaneOccluderRow(y, x0, x1, depthQ)
				}
				continue
			}
			for y := y0; y <= y1; y++ {
				for _, sp := range clipSpans {
					l := sp.L
					r := sp.R
					if l < x0 {
						l = x0
					}
					if r > x1 {
						r = x1
					}
					g.appendBillboardPlaneOccluderRow(y, l, r, depthQ)
				}
			}
			continue
		}
		for y := y0; y <= y1; y++ {
			row := y * g.viewW
			if len(clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, row, x0, x1) {
				continue
			}
			rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, clipSpans, g.solidClipScratch[:0])
			g.solidClipScratch = rowSpans
			for _, sp := range rowSpans {
				g.appendBillboardPlaneOccluderRow(y, sp.L, sp.R, depthQ)
			}
		}
	}
}

func (g *game) appendProjectedOpaqueRectPlaneOccluders(rects []projectedOpaqueRect, depthQ uint16, clipSpans []solidSpan) {
	if g == nil || len(rects) == 0 || g.viewW <= 0 || g.viewH <= 0 {
		return
	}
	for _, rect := range rects {
		x0, x1, y0, y1 := rect.x0(), rect.x1(), rect.y0(), rect.y1()
		if g.billboardClippingEnabled() && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		if !g.billboardClippingEnabled() {
			if len(clipSpans) == 0 {
				for y := y0; y <= y1; y++ {
					g.appendBillboardPlaneOccluderRow(y, x0, x1, depthQ)
				}
				continue
			}
			for y := y0; y <= y1; y++ {
				for _, sp := range clipSpans {
					l := sp.L
					r := sp.R
					if l < x0 {
						l = x0
					}
					if r > x1 {
						r = x1
					}
					g.appendBillboardPlaneOccluderRow(y, l, r, depthQ)
				}
			}
			continue
		}
		for y := y0; y <= y1; y++ {
			row := y * g.viewW
			if len(clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, row, x0, x1) {
				continue
			}
			rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, clipSpans, g.solidClipScratch[:0])
			g.solidClipScratch = rowSpans
			for _, sp := range rowSpans {
				g.appendBillboardPlaneOccluderRow(y, sp.L, sp.R, depthQ)
			}
		}
	}
}

func (g *game) buildBillboardPlaneOccludersFromQueue() {
	rows := g.ensureBillboardPlaneOccluderRows()
	if len(rows) == 0 {
		return
	}
	for _, qi := range g.billboardQueueScratch {
		projectedOpaque := g.projectedOpaqueRectScratch[qi.opaqueRectStart : qi.opaqueRectStart+qi.opaqueRectCount]
		switch qi.kind {
		case billboardQueueProjectiles:
			if qi.tex == nil || qi.tex.Height <= 0 || qi.tex.Width <= 0 {
				continue
			}
			if len(projectedOpaque) > 0 {
				g.appendProjectedOpaqueRectPlaneOccluders(projectedOpaque, qi.depthQ, qi.clipSpans)
				continue
			}
			if !qi.hasOpaque || len(qi.opaque.rects) == 0 {
				continue
			}
			g.appendBillboardOpaqueRectPlaneOccluders(qi.opaque.rects, qi.tex.Width, false, qi.dstX, qi.dstY, qi.scale, qi.clipTop, qi.clipBottom, qi.depthQ, qi.clipSpans)
		case billboardQueueMonsters:
			if qi.shadow || qi.tex == nil || qi.tex.Height <= 0 || qi.tex.Width <= 0 {
				continue
			}
			if len(projectedOpaque) > 0 {
				g.appendProjectedOpaqueRectPlaneOccluders(projectedOpaque, qi.depthQ, qi.clipSpans)
				continue
			}
			if !qi.hasOpaque || len(qi.opaque.rects) == 0 {
				continue
			}
			g.appendBillboardOpaqueRectPlaneOccluders(qi.opaque.rects, qi.tex.Width, qi.flip, qi.dstX, qi.dstY, qi.scale, qi.clipTop, qi.clipBottom, qi.depthQ, qi.clipSpans)
		case billboardQueueWorldThings:
			if qi.tex == nil || qi.tex.Height <= 0 || qi.tex.Width <= 0 {
				continue
			}
			if len(projectedOpaque) > 0 {
				g.appendProjectedOpaqueRectPlaneOccluders(projectedOpaque, qi.depthQ, qi.clipSpans)
				continue
			}
			if !qi.hasOpaque || len(qi.opaque.rects) == 0 {
				continue
			}
			g.appendBillboardOpaqueRectPlaneOccluders(qi.opaque.rects, qi.tex.Width, false, qi.dstX, qi.dstY, qi.scale, qi.clipTop, qi.clipBottom, qi.depthQ, qi.clipSpans)
		case billboardQueuePuffs:
			if qi.tex == nil || qi.tex.Height <= 0 || qi.tex.Width <= 0 {
				continue
			}
			if len(projectedOpaque) > 0 {
				g.appendProjectedOpaqueRectPlaneOccluders(projectedOpaque, qi.depthQ, qi.clipSpans)
				continue
			}
			if !qi.hasOpaque || len(qi.opaque.rects) == 0 {
				continue
			}
			g.appendBillboardOpaqueRectPlaneOccluders(qi.opaque.rects, qi.tex.Width, false, qi.dstX, qi.dstY, qi.scale, qi.clipTop, qi.clipBottom, qi.depthQ, qi.clipSpans)
		}
	}
}
