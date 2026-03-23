package doomrand

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

// Table is Doom's original 256-byte pseudo-random lookup table.
var table = [256]uint8{
	0, 8, 109, 220, 222, 241, 149, 107, 75, 248, 254, 140, 16, 66,
	74, 21, 211, 47, 80, 242, 154, 27, 205, 128, 161, 89, 77, 36,
	95, 110, 85, 48, 212, 140, 211, 249, 22, 79, 200, 50, 28, 188,
	52, 140, 202, 120, 68, 145, 62, 70, 184, 190, 91, 197, 152, 224,
	149, 104, 25, 178, 252, 182, 202, 182, 141, 197, 4, 81, 181, 242,
	145, 42, 39, 227, 156, 198, 225, 193, 219, 93, 122, 175, 249, 0,
	175, 143, 70, 239, 46, 246, 163, 53, 163, 109, 168, 135, 2, 235,
	25, 92, 20, 145, 138, 77, 69, 166, 78, 176, 173, 212, 166, 113,
	94, 161, 41, 50, 239, 49, 111, 164, 70, 60, 2, 37, 171, 75,
	136, 156, 11, 56, 42, 146, 138, 229, 73, 146, 77, 61, 98, 196,
	135, 106, 63, 197, 195, 86, 96, 203, 113, 101, 170, 247, 181, 113,
	80, 250, 108, 7, 255, 237, 129, 226, 79, 107, 112, 166, 103, 241,
	24, 223, 239, 120, 198, 58, 60, 82, 128, 3, 184, 66, 143, 224,
	145, 224, 81, 206, 163, 45, 63, 90, 168, 114, 59, 33, 159, 95,
	28, 139, 123, 98, 125, 196, 15, 70, 194, 253, 54, 14, 109, 226,
	71, 17, 161, 93, 186, 87, 244, 138, 20, 52, 123, 251, 26, 36,
	17, 46, 52, 231, 232, 76, 31, 221, 84, 37, 216, 165, 212, 106,
	197, 242, 98, 43, 39, 175, 254, 145, 190, 84, 118, 222, 187, 136,
	120, 163, 236, 249,
}

// RNG replicates Doom's original dual-index lookup-table RNG state.
type RNG struct {
	rndIndex  int
	prndIndex int
}

// New returns a cleared RNG state equivalent to M_ClearRandom.
func New() *RNG {
	return &RNG{}
}

// PRandom returns the next deterministic play-simulation random byte [0,255].
func (r *RNG) PRandom() int {
	r.prndIndex = (r.prndIndex + 1) & 0xff
	v := int(table[r.prndIndex])
	debugLogCaller("PRandom", r.rndIndex, r.prndIndex, v)
	return v
}

// MRandom returns the next menu/misc random byte [0,255].
func (r *RNG) MRandom() int {
	r.rndIndex = (r.rndIndex + 1) & 0xff
	v := int(table[r.rndIndex])
	debugLogCaller("MRandom", r.rndIndex, r.prndIndex, v)
	return v
}

// PRandomOffset returns a play-random value at the given offset without advancing state.
// offset=0 is the value that a direct call to PRandom would return next.
func (r *RNG) PRandomOffset(offset int) int {
	return int(table[(r.prndIndex+1+offset)&0xff])
}

// MRandomOffset returns a menu-random value at the given offset without advancing state.
// offset=0 is the value that a direct call to MRandom would return next.
func (r *RNG) MRandomOffset(offset int) int {
	return int(table[(r.rndIndex+1+offset)&0xff])
}

// Clear resets both indices to Doom's original zero state.
func (r *RNG) Clear() {
	r.rndIndex = 0
	r.prndIndex = 0
}

// SetState restores explicit menu and play random indices.
func (r *RNG) SetState(rndIndex, prndIndex int) {
	r.rndIndex = rndIndex & 0xff
	r.prndIndex = prndIndex & 0xff
}

// State returns current menu and play random indices.
func (r *RNG) State() (rndIndex, prndIndex int) {
	return r.rndIndex, r.prndIndex
}

var global = New()
var debugTic = -1

// PRandom is the package-level Doom-compatible P_Random.
func PRandom() int {
	return global.PRandom()
}

// MRandom is the package-level Doom-compatible M_Random.
func MRandom() int {
	return global.MRandom()
}

// PRandomOffset returns a package-level play-random value at offset without advancing state.
func PRandomOffset(offset int) int {
	return global.PRandomOffset(offset)
}

// MRandomOffset returns a package-level menu-random value at offset without advancing state.
func MRandomOffset(offset int) int {
	return global.MRandomOffset(offset)
}

// Clear resets package-level RNG indices (M_ClearRandom behavior).
func Clear() {
	global.Clear()
}

// SetState restores package-level RNG indices.
func SetState(rndIndex, prndIndex int) {
	global.SetState(rndIndex, prndIndex)
}

// State returns package-level RNG indices.
func State() (rndIndex, prndIndex int) {
	return global.State()
}

// SetDebugTic updates the current gameplay tic used by optional RNG caller logging filters.
func SetDebugTic(tic int) {
	debugTic = tic
}

func debugLogCaller(kind string, rndIndex, prndIndex, value int) {
	if os.Getenv("GD_DEBUG_RNG_CALLERS") == "" {
		return
	}
	if want := os.Getenv("GD_DEBUG_RNG_TIC"); want != "" {
		if tic, err := strconv.Atoi(want); err == nil && debugTic != tic {
			return
		}
	}
	for skip := 2; skip < 16; skip++ {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		name := "<unknown>"
		if fn := runtime.FuncForPC(pc); fn != nil {
			name = fn.Name()
		}
		if name == "" || name == "<unknown>" {
			continue
		}
		if len(name) >= len("gddoom/internal/doomrand.") && name[:len("gddoom/internal/doomrand.")] == "gddoom/internal/doomrand." {
			continue
		}
		fmt.Printf("doomrand-debug kind=%s rnd=%d prnd=%d value=%d caller=%s file=%s:%d\n", kind, rndIndex, prndIndex, value, name, file, line)
		return
	}
	fmt.Printf("doomrand-debug kind=%s rnd=%d prnd=%d value=%d caller=<unknown>\n", kind, rndIndex, prndIndex, value)
}
