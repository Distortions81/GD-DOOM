package demo

import "encoding/binary"

// GameOptions is the Boom 2.01/2.02 G_ReadOptions block. Later MBF bytes are
// retained in Raw but are not interpreted as MBF features for these versions.
type GameOptions struct {
	Raw                                                                           [64]byte
	MonstersRemember, VariableFriction, WeaponRecoil, AllowPushers, PlayerBobbing bool
	Respawn, Fast, NoMonsters, Insurance                                          bool
	Seed                                                                          uint32
}

func DecodeGameOptions(raw [64]byte) GameOptions {
	return GameOptions{Raw: raw, MonstersRemember: raw[0] != 0, VariableFriction: raw[1] != 0, WeaponRecoil: raw[2] != 0, AllowPushers: raw[3] != 0, PlayerBobbing: raw[5] != 0, Respawn: raw[6] != 0, Fast: raw[7] != 0, NoMonsters: raw[8] != 0, Insurance: raw[9] != 0, Seed: binary.BigEndian.Uint32(raw[10:14])}
}
func (h Header) GameOptions() (GameOptions, bool) {
	if h.Version != VersionBoom201 && h.Version != VersionBoom202 {
		return GameOptions{}, false
	}
	return DecodeGameOptions(h.BoomOptions), true
}

func (o GameOptions) Bytes() [64]byte {
	raw := o.Raw
	flags := []struct {
		index int
		on    bool
	}{{0, o.MonstersRemember}, {1, o.VariableFriction}, {2, o.WeaponRecoil}, {3, o.AllowPushers}, {5, o.PlayerBobbing}, {6, o.Respawn}, {7, o.Fast}, {8, o.NoMonsters}, {9, o.Insurance}}
	for _, flag := range flags {
		raw[flag.index] = 0
		if flag.on {
			raw[flag.index] = 1
		}
	}
	binary.BigEndian.PutUint32(raw[10:14], o.Seed)
	return raw
}
