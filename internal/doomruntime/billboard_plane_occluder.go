package doomruntime

func flipSpriteOpaqueRectX(rect spriteOpaqueRect, texW int) spriteOpaqueRect {
	if texW <= 0 {
		return rect
	}
	minX := texW - 1 - int(rect.maxX)
	maxX := texW - 1 - int(rect.minX)
	rect.minX = int16(minX)
	rect.maxX = int16(maxX)
	return rect
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
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, clipTop, clipBottom, g.viewW, g.viewH)
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

func (g *game) buildBillboardPlaneOccludersFromQueue() {
	rows := g.ensureBillboardPlaneOccluderRows()
	if len(rows) == 0 {
		return
	}
	for _, qi := range g.billboardQueueScratch {
		switch qi.kind {
		case billboardQueueProjectiles:
			if qi.idx < 0 || qi.idx >= len(g.projectileItemsScratch) {
				continue
			}
			it := g.projectileItemsScratch[qi.idx]
			if !it.hasSprite || !it.hasOpaque || len(it.opaque.rects) == 0 || it.spriteTex.Height <= 0 || it.spriteTex.Width <= 0 {
				continue
			}
			scale := it.h / float64(it.spriteTex.Height)
			if scale <= 0 {
				continue
			}
			dstX := it.sx - float64(it.spriteTex.OffsetX)*scale
			dstY := it.yb - float64(it.spriteTex.OffsetY)*scale
			g.appendBillboardOpaqueRectPlaneOccluders(it.opaque.rects, it.spriteTex.Width, false, dstX, dstY, scale, it.clipTop, it.clipBottom, encodeDepthQ(it.dist), it.clipSpans)
		case billboardQueueMonsters:
			if qi.idx < 0 || qi.idx >= len(g.monsterItemsScratch) {
				continue
			}
			it := g.monsterItemsScratch[qi.idx]
			if it.shadow || !it.hasOpaque || len(it.opaque.rects) == 0 || it.tex.Height <= 0 || it.tex.Width <= 0 {
				continue
			}
			scale := it.h / float64(it.tex.Height)
			if scale <= 0 {
				continue
			}
			dstX := it.sx - float64(it.tex.OffsetX)*scale
			dstY := floorSpriteTop(float64(it.tex.Height)*scale, it.yb)
			g.appendBillboardOpaqueRectPlaneOccluders(it.opaque.rects, it.tex.Width, it.flip, dstX, dstY, scale, it.clipTop, it.clipBottom, encodeDepthQ(it.dist), it.clipSpans)
		case billboardQueueWorldThings:
			if qi.idx < 0 || qi.idx >= len(g.thingItemsScratch) {
				continue
			}
			it := g.thingItemsScratch[qi.idx]
			if !it.hasOpaque || len(it.opaque.rects) == 0 || it.tex.Height <= 0 || it.tex.Width <= 0 {
				continue
			}
			scale := it.h / float64(it.tex.Height)
			if scale <= 0 {
				continue
			}
			dstX := it.sx - float64(it.tex.OffsetX)*scale
			dstY := floorSpriteTop(float64(it.tex.Height)*scale, it.yb)
			g.appendBillboardOpaqueRectPlaneOccluders(it.opaque.rects, it.tex.Width, false, dstX, dstY, scale, it.clipTop, it.clipBottom, encodeDepthQ(it.dist), it.clipSpans)
		}
	}
}
