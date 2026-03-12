package scene

const DepthQuantScale = 16.0

func EncodeDepthBiasQ(bias float64) uint16 {
	if bias <= 0 {
		return 0
	}
	scaled := bias * DepthQuantScale
	q := int(scaled)
	if float64(q) < scaled {
		q++
	}
	if q <= 0 {
		return 0
	}
	if q >= 0xFFFF {
		return 0xFFFF
	}
	return uint16(q)
}

func AddDepthQ(a, b uint16) uint16 {
	sum := uint32(a) + uint32(b)
	if sum >= 0xFFFF {
		return 0xFFFF
	}
	return uint16(sum)
}

func PackDepthStamped(depth, stamp uint16) uint32 {
	return (uint32(stamp) << 16) | uint32(depth)
}

func UnpackDepthStamp(v uint32) uint16 {
	return uint16(v >> 16)
}

func UnpackDepthQ(v uint32) uint16 {
	return uint16(v & 0xFFFF)
}

func SpriteOccludedDepthQAt(depthPix, depthPlanePix []uint32, stamp, depthQ, planeBiasQ uint16, idx int) bool {
	if len(depthPix) == 0 {
		return false
	}
	if idx < 0 || idx >= len(depthPix) {
		return true
	}
	if cur := depthPix[idx]; UnpackDepthStamp(cur) == stamp && depthQ > UnpackDepthQ(cur) {
		return true
	}
	if idx < len(depthPlanePix) {
		if cur := depthPlanePix[idx]; UnpackDepthStamp(cur) == stamp {
			threshold := AddDepthQ(UnpackDepthQ(cur), planeBiasQ)
			if depthQ > threshold {
				return true
			}
		}
	}
	return false
}
