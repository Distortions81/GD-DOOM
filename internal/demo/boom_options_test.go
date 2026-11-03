package demo

import "testing"

func TestBoomGameOptionsDecodeAndEncode(t *testing.T) {
	for _, version := range []byte{VersionBoom201, VersionBoom202} {
		data := boomDemoBytes()
		data[0] = version
		var raw [64]byte
		for _, i := range []int{0, 1, 2, 3, 5, 6, 7, 8, 9} {
			raw[i] = 1
		}
		raw[10], raw[11], raw[12], raw[13] = 0x89, 0xab, 0xcd, 0xef
		raw[4], raw[63] = 0x72, 0xff
		copy(data[13:77], raw[:])
		script, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := script.Header.GameOptions()
		if !ok || got.Seed != 0x89abcdef || !got.Insurance || !got.MonstersRemember || !got.VariableFriction || !got.WeaponRecoil || !got.AllowPushers || !got.PlayerBobbing || !script.Header.Respawn || !script.Header.Fast || !script.Header.NoMonsters || got.Bytes() != raw {
			t.Fatalf("incorrect decoded options: %+v", got)
		}
		got.Seed = 0
		got.Insurance = false
		encoded := got.Bytes()
		if encoded[4] != 0x72 || encoded[63] != 0xff || encoded[9] != 0 || encoded[10] != 0 || encoded[13] != 0 {
			t.Fatal("option edit lost reserved bytes or seed endianness")
		}
	}
	if _, ok := (Header{Version: Version109}).GameOptions(); ok {
		t.Fatal("vanilla demo selected Boom options")
	}
}
