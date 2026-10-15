package hud

// FlashPaletteIndex matches ST_doPaletteStuff, including its +1 red/gold
// palette offsets (even the faintest flash selects the second colored row).
func FlashPaletteIndex(damage, bonus, strength, suit int) int {
	count := damage
	if strength > 0 {
		count = max(count, 12-(strength>>6))
	}
	if count > 0 {
		return 1 + min(7, (count+7)>>3)
	}
	if bonus > 0 {
		return 9 + min(3, (bonus+7)>>3)
	}
	if suit > 128 || suit&8 != 0 {
		return 13
	}
	return 0
}
