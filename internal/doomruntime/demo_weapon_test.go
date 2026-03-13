package doomruntime

import "testing"

func TestDemoTraceWeaponIDIncludesSuperShotgun(t *testing.T) {
	if got := demoTraceWeaponID(weaponSuperShotgun); got != 8 {
		t.Fatalf("demoTraceWeaponID(super shotgun)=%d want=8", got)
	}
}
