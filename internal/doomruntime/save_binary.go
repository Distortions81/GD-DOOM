package doomruntime

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview"

	"github.com/zeebo/blake3"
)

const snapshotChecksumSize = 32

func encodeSnapshot(magic []byte, file saveFile) ([]byte, error) {
	body := bytes.NewBuffer(make([]byte, 0, 4096))
	w := saveBinaryWriter{buf: body}
	if err := w.saveFile(file); err != nil {
		return nil, err
	}
	sum := blake3.Sum256(body.Bytes())
	out := bytes.NewBuffer(make([]byte, 0, len(magic)+snapshotChecksumSize+body.Len()))
	if _, err := out.Write(magic); err != nil {
		return nil, err
	}
	if _, err := out.Write(sum[:]); err != nil {
		return nil, err
	}
	if _, err := out.Write(body.Bytes()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func decodeSnapshot(data []byte, magic []byte) (saveFile, error) {
	if len(data) < len(magic)+snapshotChecksumSize || !bytes.Equal(data[:len(magic)], magic) {
		return saveFile{}, errBadSaveMagic
	}
	checksumStart := len(magic)
	bodyStart := checksumStart + snapshotChecksumSize
	want := data[checksumStart:bodyStart]
	got := blake3.Sum256(data[bodyStart:])
	if !bytes.Equal(got[:], want) {
		return saveFile{}, errBadSaveChecksum
	}
	r := saveBinaryReader{r: bytes.NewReader(data[bodyStart:])}
	file, err := r.saveFile()
	if err != nil {
		return saveFile{}, err
	}
	if r.remaining() != 0 {
		return saveFile{}, fmt.Errorf("unexpected trailing snapshot data")
	}
	return file, nil
}

type saveBinaryWriter struct {
	buf *bytes.Buffer
}

func (w saveBinaryWriter) bytes(v []byte) error {
	_, err := w.buf.Write(v)
	return err
}

func (w saveBinaryWriter) u8(v byte) error { return w.buf.WriteByte(v) }
func (w saveBinaryWriter) bool(v bool) error {
	if v {
		return w.u8(1)
	}
	return w.u8(0)
}
func (w saveBinaryWriter) u16(v uint16) error { return binary.Write(w.buf, binary.LittleEndian, v) }
func (w saveBinaryWriter) i16(v int16) error  { return binary.Write(w.buf, binary.LittleEndian, v) }
func (w saveBinaryWriter) u32(v uint32) error { return binary.Write(w.buf, binary.LittleEndian, v) }
func (w saveBinaryWriter) i32(v int32) error  { return binary.Write(w.buf, binary.LittleEndian, v) }
func (w saveBinaryWriter) u64(v uint64) error { return binary.Write(w.buf, binary.LittleEndian, v) }
func (w saveBinaryWriter) i64(v int64) error  { return binary.Write(w.buf, binary.LittleEndian, v) }

func (w saveBinaryWriter) int(v int) error {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return fmt.Errorf("int out of range: %d", v)
	}
	return w.i32(int32(v))
}

func (w saveBinaryWriter) f64(v float64) error { return w.u64(math.Float64bits(v)) }

func (w saveBinaryWriter) str(v string) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	_, err := w.buf.WriteString(v)
	return err
}

func (w saveBinaryWriter) bytesWithLen(v []byte) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	_, err := w.buf.Write(v)
	return err
}

func (w saveBinaryWriter) mapName(v mapdata.MapName) error { return w.str(string(v)) }

func (w saveBinaryWriter) viewState(v mapview.ViewState) error {
	values := []float64{
		v.CamX, v.CamY, v.Zoom, v.FitZoom,
		v.SavedView.CamX, v.SavedView.CamY, v.SavedView.Zoom,
		v.PrevCamX, v.PrevCamY, v.RenderCamX, v.RenderCamY,
	}
	for _, f := range values {
		if err := w.f64(f); err != nil {
			return err
		}
	}
	for _, b := range []bool{v.FollowMode, v.SavedView.Follow, v.SavedView.Valid} {
		if err := w.bool(b); err != nil {
			return err
		}
	}
	return nil
}

func (w saveBinaryWriter) playerStats(v playerStats) error {
	for _, n := range []int{v.Health, v.Armor, v.ArmorType, v.Bullets, v.Shells, v.Rockets, v.Cells} {
		if err := w.int(n); err != nil {
			return err
		}
	}
	return nil
}

func (w saveBinaryWriter) playerSaveState(v playerSaveState) error {
	for _, n := range []int64{v.X, v.Y, v.Z, v.FloorZ, v.CeilZ, v.MomX, v.MomY, v.MomZ, v.ViewHeight, v.DeltaViewHeight} {
		if err := w.i64(n); err != nil {
			return err
		}
	}
	for _, n := range []int{v.Subsector, v.Sector, v.ReactionTime} {
		if err := w.int(n); err != nil {
			return err
		}
	}
	return w.u32(v.Angle)
}

func (w saveBinaryWriter) playerInventorySaveState(v playerInventorySaveState) error {
	for _, b := range []bool{v.BlueKey, v.RedKey, v.YellowKey, v.Backpack, v.Strength, v.AllMap} {
		if err := w.bool(b); err != nil {
			return err
		}
	}
	for _, n := range []int{v.StrengthCount, v.InvulnTics, v.InvisTics, v.RadSuitTics, v.LightAmpTics, v.ReadyWeapon, v.PendingWeapon} {
		if err := w.int(n); err != nil {
			return err
		}
	}
	keys := make([]int, 0, len(v.Weapons))
	for key, owned := range v.Weapons {
		if owned {
			keys = append(keys, int(key))
		}
	}
	sort.Ints(keys)
	if err := w.u32(uint32(len(keys))); err != nil {
		return err
	}
	for _, key := range keys {
		if err := w.i16(int16(key)); err != nil {
			return err
		}
	}
	return nil
}

func (w saveBinaryWriter) mapData(v *mapdata.Map) error {
	if err := w.bool(v != nil); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	if err := w.mapName(v.Name); err != nil {
		return err
	}
	if err := w.u32(uint32(len(v.Things))); err != nil {
		return err
	}
	for _, it := range v.Things {
		for _, n := range []int16{it.X, it.Y, it.Angle, it.Type, it.Flags} {
			if err := w.i16(n); err != nil {
				return err
			}
		}
	}
	if err := w.u32(uint32(len(v.Vertexes))); err != nil {
		return err
	}
	for _, it := range v.Vertexes {
		if err := w.i16(it.X); err != nil {
			return err
		}
		if err := w.i16(it.Y); err != nil {
			return err
		}
	}
	if err := w.u32(uint32(len(v.Linedefs))); err != nil {
		return err
	}
	for _, it := range v.Linedefs {
		if err := w.u16(it.V1); err != nil {
			return err
		}
		if err := w.u16(it.V2); err != nil {
			return err
		}
		if err := w.u16(it.Flags); err != nil {
			return err
		}
		if err := w.u16(it.Special); err != nil {
			return err
		}
		if err := w.u16(it.Tag); err != nil {
			return err
		}
		for _, n := range it.SideNum {
			if err := w.i16(n); err != nil {
				return err
			}
		}
	}
	if err := w.u32(uint32(len(v.Sidedefs))); err != nil {
		return err
	}
	for _, it := range v.Sidedefs {
		if err := w.i16(it.TextureOffset); err != nil {
			return err
		}
		if err := w.i16(it.RowOffset); err != nil {
			return err
		}
		for _, s := range []string{it.Top, it.Bottom, it.Mid} {
			if err := w.str(s); err != nil {
				return err
			}
		}
		if err := w.u16(it.Sector); err != nil {
			return err
		}
	}
	if err := w.u32(uint32(len(v.Sectors))); err != nil {
		return err
	}
	for _, it := range v.Sectors {
		if err := w.i16(it.FloorHeight); err != nil {
			return err
		}
		if err := w.i16(it.CeilingHeight); err != nil {
			return err
		}
		if err := w.str(it.FloorPic); err != nil {
			return err
		}
		if err := w.str(it.CeilingPic); err != nil {
			return err
		}
		if err := w.i16(it.Light); err != nil {
			return err
		}
		if err := w.i16(it.Special); err != nil {
			return err
		}
		if err := w.i16(it.Tag); err != nil {
			return err
		}
	}
	if err := w.u32(uint32(len(v.Segs))); err != nil {
		return err
	}
	for _, it := range v.Segs {
		for _, n := range []uint16{it.StartVertex, it.EndVertex, it.Angle, it.Linedef, it.Direction, it.Offset} {
			if err := w.u16(n); err != nil {
				return err
			}
		}
	}
	if err := w.u32(uint32(len(v.SubSectors))); err != nil {
		return err
	}
	for _, it := range v.SubSectors {
		if err := w.u16(it.SegCount); err != nil {
			return err
		}
		if err := w.u16(it.FirstSeg); err != nil {
			return err
		}
	}
	if err := w.u32(uint32(len(v.Nodes))); err != nil {
		return err
	}
	for _, it := range v.Nodes {
		for _, n := range []int16{it.X, it.Y, it.DX, it.DY} {
			if err := w.i16(n); err != nil {
				return err
			}
		}
		for _, arr := range [][]int16{it.BBoxR[:], it.BBoxL[:]} {
			for _, n := range arr {
				if err := w.i16(n); err != nil {
					return err
				}
			}
		}
		for _, n := range it.ChildID {
			if err := w.u16(n); err != nil {
				return err
			}
		}
	}
	if err := w.bytesWithLen(v.Reject); err != nil {
		return err
	}
	if err := w.bool(v.RejectMatrix != nil); err != nil {
		return err
	}
	if v.RejectMatrix != nil {
		if err := w.int(v.RejectMatrix.SectorCount); err != nil {
			return err
		}
		if err := w.bytesWithLen(v.RejectMatrix.Data); err != nil {
			return err
		}
	}
	if err := w.u32(uint32(len(v.Blockmap))); err != nil {
		return err
	}
	for _, n := range v.Blockmap {
		if err := w.i16(n); err != nil {
			return err
		}
	}
	if err := w.bool(v.BlockMap != nil); err != nil {
		return err
	}
	if v.BlockMap != nil {
		for _, n := range []int16{v.BlockMap.OriginX, v.BlockMap.OriginY, v.BlockMap.Width, v.BlockMap.Height} {
			if err := w.i16(n); err != nil {
				return err
			}
		}
		if err := w.u32(uint32(len(v.BlockMap.Offsets))); err != nil {
			return err
		}
		for _, n := range v.BlockMap.Offsets {
			if err := w.u16(n); err != nil {
				return err
			}
		}
		if err := w.u32(uint32(len(v.BlockMap.Cells))); err != nil {
			return err
		}
		for _, cell := range v.BlockMap.Cells {
			if err := w.u32(uint32(len(cell))); err != nil {
				return err
			}
			for _, n := range cell {
				if err := w.i16(n); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (w saveBinaryWriter) gameSaveState(v gameSaveState) error {
	if err := w.playerSaveState(v.Player); err != nil {
		return err
	}
	if err := w.viewState(v.View); err != nil {
		return err
	}
	for _, n := range []int{v.Mode, v.ParityReveal, v.ParityIDDT, v.GammaLevel, v.UseFlash} {
		if err := w.int(n); err != nil {
			return err
		}
	}
	for _, b := range []bool{v.RotateView, v.ShowGrid, v.ShowLegend, v.PaletteLUTEnabled, v.CRTEnabled, v.HUDMessagesEnabled} {
		if err := w.bool(b); err != nil {
			return err
		}
	}
	if err := w.str(v.UseText); err != nil {
		return err
	}
	for _, n := range []int64{v.PrevPX, v.PrevPY, v.PlayerViewZ} {
		if err := w.i64(n); err != nil {
			return err
		}
	}
	if err := w.u32(v.PrevAngle); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingCollected); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingDropped); err != nil {
		return err
	}
	if err := w.i64Slice(v.ThingX); err != nil {
		return err
	}
	if err := w.i64Slice(v.ThingY); err != nil {
		return err
	}
	if err := w.u32Slice(v.ThingAngleState); err != nil {
		return err
	}
	if err := w.i64Slice(v.ThingZState); err != nil {
		return err
	}
	if err := w.i64Slice(v.ThingFloorState); err != nil {
		return err
	}
	if err := w.i64Slice(v.ThingCeilState); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingSupportValid); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingHP); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingAggro); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingTargetPlayer); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingTargetIdx); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingThreshold); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingCooldown); err != nil {
		return err
	}
	if err := w.u8Slice(v.ThingMoveDir); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingMoveCount); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingJustAtk); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingJustHit); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingReactionTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingWakeTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingLastLook); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingDead); err != nil {
		return err
	}
	if err := w.boolSlice(v.ThingGibbed); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingGibTick); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingDeathTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingAttackTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingAttackPhase); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingAttackFireTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingPainTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingThinkWait); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingDoomState); err != nil {
		return err
	}
	if err := w.u8Slice(v.ThingState); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingStateTics); err != nil {
		return err
	}
	if err := w.intSlice(v.ThingStatePhase); err != nil {
		return err
	}
	if err := w.bossSpawnCubeSlice(v.BossSpawnCubes); err != nil {
		return err
	}
	if err := w.bossSpawnFireSlice(v.BossSpawnFires); err != nil {
		return err
	}
	if err := w.int(v.BossBrainTargetOrder); err != nil {
		return err
	}
	if err := w.bool(v.BossBrainEasyToggle); err != nil {
		return err
	}
	if err := w.projectileSlice(v.Projectiles); err != nil {
		return err
	}
	if err := w.projectileImpactSlice(v.ProjectileImpacts); err != nil {
		return err
	}
	if err := w.hitscanPuffSlice(v.HitscanPuffs); err != nil {
		return err
	}
	for _, n := range []int{v.CheatLevel, v.WeaponState, v.WeaponStateTics, v.WeaponFlashState, v.WeaponFlashTics, v.WeaponPSpriteY, v.WorldTic, v.SecretsFound, v.SecretsTotal, v.PlayerMobjHealth, v.DamageFlashTic, v.BonusFlashTic} {
		if err := w.int(n); err != nil {
			return err
		}
	}
	for _, b := range []bool{v.Invulnerable, v.NoClip, v.AlwaysRun, v.AutoWeaponSwitch, v.WeaponRefire, v.WeaponAttackDown, v.IsDead} {
		if err := w.bool(b); err != nil {
			return err
		}
	}
	if err := w.playerInventorySaveState(v.Inventory); err != nil {
		return err
	}
	if err := w.playerStats(v.Stats); err != nil {
		return err
	}
	if err := w.boolSlice(v.SecretFound); err != nil {
		return err
	}
	if err := w.boolSlice(v.SectorSoundTarget); err != nil {
		return err
	}
	if err := w.sectorLightEffectSlice(v.SectorLightFx); err != nil {
		return err
	}
	if err := w.i64Slice(v.SectorFloor); err != nil {
		return err
	}
	if err := w.i64Slice(v.SectorCeil); err != nil {
		return err
	}
	if err := w.u16Slice(v.LineSpecial); err != nil {
		return err
	}
	if err := w.doorThinkerMap(v.Doors); err != nil {
		return err
	}
	if err := w.floorThinkerMap(v.Floors); err != nil {
		return err
	}
	if err := w.platThinkerMap(v.Plats); err != nil {
		return err
	}
	if err := w.ceilingThinkerMap(v.Ceilings); err != nil {
		return err
	}
	return w.delayedSwitchTextureSlice(v.DelayedSwitchReverts)
}

func (w saveBinaryWriter) saveFile(v saveFile) error {
	if err := w.int(v.Version); err != nil {
		return err
	}
	if err := w.str(v.Description); err != nil {
		return err
	}
	if err := w.mapName(v.Current); err != nil {
		return err
	}
	if err := w.int(v.RNG.MenuIndex); err != nil {
		return err
	}
	if err := w.int(v.RNG.PlayIndex); err != nil {
		return err
	}
	return w.gameSaveState(v.Game)
}

func (w saveBinaryWriter) boolSlice(v []bool) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.bool(it); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) intSlice(v []int) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.int(it); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) i64Slice(v []int64) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.i64(it); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) u32Slice(v []uint32) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.u32(it); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) u16Slice(v []uint16) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.u16(it); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) u8Slice(v []uint8) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	_, err := w.buf.Write(v)
	return err
}

func (w saveBinaryWriter) bossSpawnCubeSlice(v []bossSpawnCubeSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int64{it.X, it.Y, it.Z, it.VX, it.VY, it.VZ} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		for _, n := range []int{it.TargetIdx, it.StateTics, it.StateStep, it.Reaction} {
			if err := w.int(n); err != nil {
				return err
			}
		}
	}
	return nil
}
func (w saveBinaryWriter) bossSpawnFireSlice(v []bossSpawnFireSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int64{it.X, it.Y, it.Z} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		if err := w.int(it.Tics); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) projectileSlice(v []projectileSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int64{it.X, it.Y, it.Z, it.VX, it.VY, it.VZ, it.FloorZ, it.CeilZ, it.Radius, it.Height, it.SourceX, it.SourceY} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		for _, n := range []int{it.TTL, it.SourceThing, it.Kind} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		if err := w.i16(it.SourceType); err != nil {
			return err
		}
		if err := w.bool(it.SourcePlayer); err != nil {
			return err
		}
		if err := w.bool(it.TracerPlayer); err != nil {
			return err
		}
		if err := w.u32(it.Angle); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) projectileImpactSlice(v []projectileImpactSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int64{it.X, it.Y, it.Z, it.FloorZ, it.CeilZ} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		for _, n := range []int{it.Kind, it.SourceThing, it.LastLook, it.Tics, it.TotalTics} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		if err := w.i16(it.SourceType); err != nil {
			return err
		}
		if err := w.bool(it.SourcePlayer); err != nil {
			return err
		}
		if err := w.u32(it.Angle); err != nil {
			return err
		}
		if err := w.bool(it.SprayDone); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) hitscanPuffSlice(v []hitscanPuffSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int64{it.X, it.Y, it.Z, it.MomZ} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		for _, n := range []int{it.Tics, it.State, it.TotalTic} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		if err := w.u8(it.Kind); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) sectorLightEffectSlice(v []sectorLightEffectSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		if err := w.u8(it.Kind); err != nil {
			return err
		}
		if err := w.i16(it.MinLight); err != nil {
			return err
		}
		if err := w.i16(it.MaxLight); err != nil {
			return err
		}
		for _, n := range []int{it.Count, it.MinTime, it.MaxTime, it.DarkTime, it.BrightTime, it.Direction} {
			if err := w.int(n); err != nil {
				return err
			}
		}
	}
	return nil
}
func (w saveBinaryWriter) delayedSwitchTextureSlice(v []delayedSwitchTextureSaveState) error {
	if err := w.u32(uint32(len(v))); err != nil {
		return err
	}
	for _, it := range v {
		for _, n := range []int{it.Line, it.Sidedef, it.Tics} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		for _, s := range []string{it.Top, it.Bottom, it.Mid} {
			if err := w.str(s); err != nil {
				return err
			}
		}
	}
	return nil
}
func (w saveBinaryWriter) doorThinkerMap(v map[int]doorThinkerSaveState) error {
	keys := sortedSaveIntKeys(v)
	if err := w.u32(uint32(len(keys))); err != nil {
		return err
	}
	for _, key := range keys {
		it := v[key]
		if err := w.int(key); err != nil {
			return err
		}
		for _, n := range []int{it.Sector, it.Type, it.Direction, it.TopWait, it.TopCountdown} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		for _, n := range []int64{it.TopHeight, it.Speed} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
	}
	return nil
}
func (w saveBinaryWriter) floorThinkerMap(v map[int]floorThinkerSaveState) error {
	keys := sortedSaveIntKeys(v)
	if err := w.u32(uint32(len(keys))); err != nil {
		return err
	}
	for _, key := range keys {
		it := v[key]
		if err := w.int(key); err != nil {
			return err
		}
		for _, n := range []int{it.Sector, it.Direction} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		for _, n := range []int64{it.Speed, it.DestHeight} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		if err := w.u8(it.Finish); err != nil {
			return err
		}
		if err := w.str(it.FinishFlat); err != nil {
			return err
		}
		if err := w.i16(it.FinishSpecial); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) platThinkerMap(v map[int]platThinkerSaveState) error {
	keys := sortedSaveIntKeys(v)
	if err := w.u32(uint32(len(keys))); err != nil {
		return err
	}
	for _, key := range keys {
		it := v[key]
		if err := w.int(key); err != nil {
			return err
		}
		if err := w.int(it.Sector); err != nil {
			return err
		}
		for _, n := range []uint8{it.Type, it.Status, it.OldStatus} {
			if err := w.u8(n); err != nil {
				return err
			}
		}
		for _, n := range []int64{it.Speed, it.Low, it.High} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		for _, n := range []int{it.Wait, it.Count} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		if err := w.str(it.FinishFlat); err != nil {
			return err
		}
		if err := w.i16(it.FinishSpecial); err != nil {
			return err
		}
	}
	return nil
}
func (w saveBinaryWriter) ceilingThinkerMap(v map[int]ceilingThinkerSaveState) error {
	keys := sortedSaveIntKeys(v)
	if err := w.u32(uint32(len(keys))); err != nil {
		return err
	}
	for _, key := range keys {
		it := v[key]
		if err := w.int(key); err != nil {
			return err
		}
		for _, n := range []int{it.Sector, it.Direction, it.OldDirection} {
			if err := w.int(n); err != nil {
				return err
			}
		}
		if err := w.str(string(it.Action)); err != nil {
			return err
		}
		for _, n := range []int64{it.Speed, it.TopHeight, it.BottomHeight} {
			if err := w.i64(n); err != nil {
				return err
			}
		}
		if err := w.bool(it.Crush); err != nil {
			return err
		}
	}
	return nil
}

func sortedSaveIntKeys[T any](m map[int]T) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

type saveBinaryReader struct {
	r *bytes.Reader
}

func (r saveBinaryReader) remaining() int { return r.r.Len() }
func (r saveBinaryReader) read(p []byte) error {
	_, err := io.ReadFull(r.r, p)
	return err
}
func (r saveBinaryReader) u8() (byte, error) {
	var b [1]byte
	err := r.read(b[:])
	return b[0], err
}
func (r saveBinaryReader) bool() (bool, error) {
	v, err := r.u8()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}
func (r saveBinaryReader) u16() (uint16, error) {
	var v uint16
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) i16() (int16, error) {
	var v int16
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) u32() (uint32, error) {
	var v uint32
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) i32() (int32, error) {
	var v int32
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) u64() (uint64, error) {
	var v uint64
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) i64() (int64, error) {
	var v int64
	err := binary.Read(r.r, binary.LittleEndian, &v)
	return v, err
}
func (r saveBinaryReader) int() (int, error) { v, err := r.i32(); return int(v), err }
func (r saveBinaryReader) f64() (float64, error) {
	v, err := r.u64()
	return math.Float64frombits(v), err
}
func (r saveBinaryReader) str() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if err := r.read(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
func (r saveBinaryReader) bytesWithLen() ([]byte, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	if err := r.read(buf); err != nil {
		return nil, err
	}
	return buf, nil
}
func (r saveBinaryReader) mapName() (mapdata.MapName, error) {
	s, err := r.str()
	return mapdata.MapName(s), err
}

func (r saveBinaryReader) viewState() (mapview.ViewState, error) {
	var v mapview.ViewState
	var err error
	readFloat := func(dst *float64) error { *dst, err = r.f64(); return err }
	for _, dst := range []*float64{&v.CamX, &v.CamY, &v.Zoom, &v.FitZoom, &v.SavedView.CamX, &v.SavedView.CamY, &v.SavedView.Zoom, &v.PrevCamX, &v.PrevCamY, &v.RenderCamX, &v.RenderCamY} {
		if err = readFloat(dst); err != nil {
			return v, err
		}
	}
	if v.FollowMode, err = r.bool(); err != nil {
		return v, err
	}
	if v.SavedView.Follow, err = r.bool(); err != nil {
		return v, err
	}
	if v.SavedView.Valid, err = r.bool(); err != nil {
		return v, err
	}
	return v, nil
}

func (r saveBinaryReader) playerStats() (playerStats, error) {
	var v playerStats
	var err error
	ints := []*int{&v.Health, &v.Armor, &v.ArmorType, &v.Bullets, &v.Shells, &v.Rockets, &v.Cells}
	for _, dst := range ints {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	return v, nil
}

func (r saveBinaryReader) playerSaveState() (playerSaveState, error) {
	var v playerSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z, &v.FloorZ, &v.CeilZ, &v.MomX, &v.MomY, &v.MomZ, &v.ViewHeight, &v.DeltaViewHeight} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.Subsector, &v.Sector, &v.ReactionTime} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	if v.Angle, err = r.u32(); err != nil {
		return v, err
	}
	return v, nil
}

func (r saveBinaryReader) playerInventorySaveState() (playerInventorySaveState, error) {
	var v playerInventorySaveState
	var err error
	for _, dst := range []*bool{&v.BlueKey, &v.RedKey, &v.YellowKey, &v.Backpack, &v.Strength, &v.AllMap} {
		if *dst, err = r.bool(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.StrengthCount, &v.InvulnTics, &v.InvisTics, &v.RadSuitTics, &v.LightAmpTics, &v.ReadyWeapon, &v.PendingWeapon} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	n, err := r.u32()
	if err != nil {
		return v, err
	}
	v.Weapons = make(map[int16]bool, n)
	for i := uint32(0); i < n; i++ {
		key, err := r.i16()
		if err != nil {
			return v, err
		}
		v.Weapons[key] = true
	}
	return v, nil
}

func (r saveBinaryReader) mapData() (*mapdata.Map, error) {
	ok, err := r.bool()
	if err != nil || !ok {
		return nil, err
	}
	v := &mapdata.Map{}
	if v.Name, err = r.mapName(); err != nil {
		return nil, err
	}
	if v.Things, err = readSlice(r, func() (mapdata.Thing, error) {
		var it mapdata.Thing
		var err error
		for _, dst := range []*int16{&it.X, &it.Y, &it.Angle, &it.Type, &it.Flags} {
			if *dst, err = r.i16(); err != nil {
				return it, err
			}
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Vertexes, err = readSlice(r, func() (mapdata.Vertex, error) {
		var it mapdata.Vertex
		var err error
		if it.X, err = r.i16(); err != nil {
			return it, err
		}
		if it.Y, err = r.i16(); err != nil {
			return it, err
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Linedefs, err = readSlice(r, func() (mapdata.Linedef, error) {
		var it mapdata.Linedef
		var err error
		if it.V1, err = r.u16(); err != nil {
			return it, err
		}
		if it.V2, err = r.u16(); err != nil {
			return it, err
		}
		if it.Flags, err = r.u16(); err != nil {
			return it, err
		}
		if it.Special, err = r.u16(); err != nil {
			return it, err
		}
		if it.Tag, err = r.u16(); err != nil {
			return it, err
		}
		for i := range it.SideNum {
			if it.SideNum[i], err = r.i16(); err != nil {
				return it, err
			}
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Sidedefs, err = readSlice(r, func() (mapdata.Sidedef, error) {
		var it mapdata.Sidedef
		if it.TextureOffset, err = r.i16(); err != nil {
			return it, err
		}
		if it.RowOffset, err = r.i16(); err != nil {
			return it, err
		}
		if it.Top, err = r.str(); err != nil {
			return it, err
		}
		if it.Bottom, err = r.str(); err != nil {
			return it, err
		}
		if it.Mid, err = r.str(); err != nil {
			return it, err
		}
		if it.Sector, err = r.u16(); err != nil {
			return it, err
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Sectors, err = readSlice(r, func() (mapdata.Sector, error) {
		var it mapdata.Sector
		if it.FloorHeight, err = r.i16(); err != nil {
			return it, err
		}
		if it.CeilingHeight, err = r.i16(); err != nil {
			return it, err
		}
		if it.FloorPic, err = r.str(); err != nil {
			return it, err
		}
		if it.CeilingPic, err = r.str(); err != nil {
			return it, err
		}
		if it.Light, err = r.i16(); err != nil {
			return it, err
		}
		if it.Special, err = r.i16(); err != nil {
			return it, err
		}
		if it.Tag, err = r.i16(); err != nil {
			return it, err
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Segs, err = readSlice(r, func() (mapdata.Seg, error) {
		var it mapdata.Seg
		if it.StartVertex, err = r.u16(); err != nil {
			return it, err
		}
		if it.EndVertex, err = r.u16(); err != nil {
			return it, err
		}
		if it.Angle, err = r.u16(); err != nil {
			return it, err
		}
		if it.Linedef, err = r.u16(); err != nil {
			return it, err
		}
		if it.Direction, err = r.u16(); err != nil {
			return it, err
		}
		if it.Offset, err = r.u16(); err != nil {
			return it, err
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.SubSectors, err = readSlice(r, func() (mapdata.SubSector, error) {
		var it mapdata.SubSector
		if it.SegCount, err = r.u16(); err != nil {
			return it, err
		}
		if it.FirstSeg, err = r.u16(); err != nil {
			return it, err
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Nodes, err = readSlice(r, func() (mapdata.Node, error) {
		var it mapdata.Node
		for _, dst := range []*int16{&it.X, &it.Y, &it.DX, &it.DY} {
			if *dst, err = r.i16(); err != nil {
				return it, err
			}
		}
		for i := range it.BBoxR {
			if it.BBoxR[i], err = r.i16(); err != nil {
				return it, err
			}
		}
		for i := range it.BBoxL {
			if it.BBoxL[i], err = r.i16(); err != nil {
				return it, err
			}
		}
		for i := range it.ChildID {
			if it.ChildID[i], err = r.u16(); err != nil {
				return it, err
			}
		}
		return it, nil
	}); err != nil {
		return nil, err
	}
	if v.Reject, err = r.bytesWithLen(); err != nil {
		return nil, err
	}
	hasRejectMatrix, err := r.bool()
	if err != nil {
		return nil, err
	}
	if hasRejectMatrix {
		v.RejectMatrix = &mapdata.RejectMatrix{}
		if v.RejectMatrix.SectorCount, err = r.int(); err != nil {
			return nil, err
		}
		if v.RejectMatrix.Data, err = r.bytesWithLen(); err != nil {
			return nil, err
		}
	}
	blockmapLen, err := r.u32()
	if err != nil {
		return nil, err
	}
	v.Blockmap = make([]int16, blockmapLen)
	for i := range v.Blockmap {
		if v.Blockmap[i], err = r.i16(); err != nil {
			return nil, err
		}
	}
	hasBlockMap, err := r.bool()
	if err != nil {
		return nil, err
	}
	if hasBlockMap {
		v.BlockMap = &mapdata.BlockMap{}
		if v.BlockMap.OriginX, err = r.i16(); err != nil {
			return nil, err
		}
		if v.BlockMap.OriginY, err = r.i16(); err != nil {
			return nil, err
		}
		if v.BlockMap.Width, err = r.i16(); err != nil {
			return nil, err
		}
		if v.BlockMap.Height, err = r.i16(); err != nil {
			return nil, err
		}
		offsetLen, err := r.u32()
		if err != nil {
			return nil, err
		}
		v.BlockMap.Offsets = make([]uint16, offsetLen)
		for i := range v.BlockMap.Offsets {
			if v.BlockMap.Offsets[i], err = r.u16(); err != nil {
				return nil, err
			}
		}
		cellLen, err := r.u32()
		if err != nil {
			return nil, err
		}
		v.BlockMap.Cells = make([][]int16, cellLen)
		for i := range v.BlockMap.Cells {
			n, err := r.u32()
			if err != nil {
				return nil, err
			}
			v.BlockMap.Cells[i] = make([]int16, n)
			for j := range v.BlockMap.Cells[i] {
				if v.BlockMap.Cells[i][j], err = r.i16(); err != nil {
					return nil, err
				}
			}
		}
	}
	return v, nil
}

func (r saveBinaryReader) saveFile() (saveFile, error) {
	var v saveFile
	var err error
	if v.Version, err = r.int(); err != nil {
		return v, err
	}
	if v.Description, err = r.str(); err != nil {
		return v, err
	}
	if v.Current, err = r.mapName(); err != nil {
		return v, err
	}
	if v.RNG.MenuIndex, err = r.int(); err != nil {
		return v, err
	}
	if v.RNG.PlayIndex, err = r.int(); err != nil {
		return v, err
	}
	if v.Game, err = r.gameSaveState(); err != nil {
		return v, err
	}
	return v, nil
}

func (r saveBinaryReader) gameSaveState() (gameSaveState, error) {
	var v gameSaveState
	var err error
	if v.Player, err = r.playerSaveState(); err != nil {
		return v, err
	}
	if v.View, err = r.viewState(); err != nil {
		return v, err
	}
	for _, dst := range []*int{&v.Mode, &v.ParityReveal, &v.ParityIDDT, &v.GammaLevel, &v.UseFlash} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*bool{&v.RotateView, &v.ShowGrid, &v.ShowLegend, &v.PaletteLUTEnabled, &v.CRTEnabled, &v.HUDMessagesEnabled} {
		if *dst, err = r.bool(); err != nil {
			return v, err
		}
	}
	if v.UseText, err = r.str(); err != nil {
		return v, err
	}
	for _, dst := range []*int64{&v.PrevPX, &v.PrevPY, &v.PlayerViewZ} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	if v.PrevAngle, err = r.u32(); err != nil {
		return v, err
	}
	if v.ThingCollected, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingDropped, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingX, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.ThingY, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.ThingAngleState, err = readU32Slice(r); err != nil {
		return v, err
	}
	if v.ThingZState, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.ThingFloorState, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.ThingCeilState, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.ThingSupportValid, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingHP, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingAggro, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingTargetPlayer, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingTargetIdx, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingThreshold, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingCooldown, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingMoveDir, err = readU8Slice(r); err != nil {
		return v, err
	}
	if v.ThingMoveCount, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingJustAtk, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingJustHit, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingReactionTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingWakeTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingLastLook, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingDead, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingGibbed, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.ThingGibTick, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingDeathTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingAttackTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingAttackPhase, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingAttackFireTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingPainTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingThinkWait, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingDoomState, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingState, err = readU8Slice(r); err != nil {
		return v, err
	}
	if v.ThingStateTics, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.ThingStatePhase, err = readIntSlice(r); err != nil {
		return v, err
	}
	if v.BossSpawnCubes, err = readSlice(r, r.bossSpawnCube); err != nil {
		return v, err
	}
	if v.BossSpawnFires, err = readSlice(r, r.bossSpawnFire); err != nil {
		return v, err
	}
	if v.BossBrainTargetOrder, err = r.int(); err != nil {
		return v, err
	}
	if v.BossBrainEasyToggle, err = r.bool(); err != nil {
		return v, err
	}
	if v.Projectiles, err = readSlice(r, r.projectile); err != nil {
		return v, err
	}
	if v.ProjectileImpacts, err = readSlice(r, r.projectileImpact); err != nil {
		return v, err
	}
	if v.HitscanPuffs, err = readSlice(r, r.hitscanPuff); err != nil {
		return v, err
	}
	for _, dst := range []*int{&v.CheatLevel, &v.WeaponState, &v.WeaponStateTics, &v.WeaponFlashState, &v.WeaponFlashTics, &v.WeaponPSpriteY, &v.WorldTic, &v.SecretsFound, &v.SecretsTotal, &v.PlayerMobjHealth, &v.DamageFlashTic, &v.BonusFlashTic} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*bool{&v.Invulnerable, &v.NoClip, &v.AlwaysRun, &v.AutoWeaponSwitch, &v.WeaponRefire, &v.WeaponAttackDown, &v.IsDead} {
		if *dst, err = r.bool(); err != nil {
			return v, err
		}
	}
	if v.Inventory, err = r.playerInventorySaveState(); err != nil {
		return v, err
	}
	if v.Stats, err = r.playerStats(); err != nil {
		return v, err
	}
	if v.SecretFound, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.SectorSoundTarget, err = readBoolSlice(r); err != nil {
		return v, err
	}
	if v.SectorLightFx, err = readSlice(r, r.sectorLightEffect); err != nil {
		return v, err
	}
	if v.SectorFloor, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.SectorCeil, err = readI64Slice(r); err != nil {
		return v, err
	}
	if v.LineSpecial, err = readU16Slice(r); err != nil {
		return v, err
	}
	if v.Doors, err = r.doorThinkerMap(); err != nil {
		return v, err
	}
	if v.Floors, err = r.floorThinkerMap(); err != nil {
		return v, err
	}
	if v.Plats, err = r.platThinkerMap(); err != nil {
		return v, err
	}
	if v.Ceilings, err = r.ceilingThinkerMap(); err != nil {
		return v, err
	}
	if v.DelayedSwitchReverts, err = readSlice(r, r.delayedSwitchTexture); err != nil {
		return v, err
	}
	return v, nil
}

func (r saveBinaryReader) bossSpawnCube() (bossSpawnCubeSaveState, error) {
	var v bossSpawnCubeSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z, &v.VX, &v.VY, &v.VZ} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.TargetIdx, &v.StateTics, &v.StateStep, &v.Reaction} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	return v, nil
}
func (r saveBinaryReader) bossSpawnFire() (bossSpawnFireSaveState, error) {
	var v bossSpawnFireSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	if v.Tics, err = r.int(); err != nil {
		return v, err
	}
	return v, nil
}
func (r saveBinaryReader) projectile() (projectileSaveState, error) {
	var v projectileSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z, &v.VX, &v.VY, &v.VZ, &v.FloorZ, &v.CeilZ, &v.Radius, &v.Height, &v.SourceX, &v.SourceY} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.TTL, &v.SourceThing, &v.Kind} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	if v.SourceType, err = r.i16(); err != nil {
		return v, err
	}
	if v.SourcePlayer, err = r.bool(); err != nil {
		return v, err
	}
	if v.TracerPlayer, err = r.bool(); err != nil {
		return v, err
	}
	if v.Angle, err = r.u32(); err != nil {
		return v, err
	}
	return v, nil
}
func (r saveBinaryReader) projectileImpact() (projectileImpactSaveState, error) {
	var v projectileImpactSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z, &v.FloorZ, &v.CeilZ} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.Kind, &v.SourceThing, &v.LastLook, &v.Tics, &v.TotalTics} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	if v.SourceType, err = r.i16(); err != nil {
		return v, err
	}
	if v.SourcePlayer, err = r.bool(); err != nil {
		return v, err
	}
	if v.Angle, err = r.u32(); err != nil {
		return v, err
	}
	if v.SprayDone, err = r.bool(); err != nil {
		return v, err
	}
	return v, nil
}
func (r saveBinaryReader) hitscanPuff() (hitscanPuffSaveState, error) {
	var v hitscanPuffSaveState
	var err error
	for _, dst := range []*int64{&v.X, &v.Y, &v.Z, &v.MomZ} {
		if *dst, err = r.i64(); err != nil {
			return v, err
		}
	}
	for _, dst := range []*int{&v.Tics, &v.State, &v.TotalTic} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	if v.Kind, err = r.u8(); err != nil {
		return v, err
	}
	return v, nil
}
func (r saveBinaryReader) sectorLightEffect() (sectorLightEffectSaveState, error) {
	var v sectorLightEffectSaveState
	var err error
	if v.Kind, err = r.u8(); err != nil {
		return v, err
	}
	if v.MinLight, err = r.i16(); err != nil {
		return v, err
	}
	if v.MaxLight, err = r.i16(); err != nil {
		return v, err
	}
	for _, dst := range []*int{&v.Count, &v.MinTime, &v.MaxTime, &v.DarkTime, &v.BrightTime, &v.Direction} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	return v, nil
}
func (r saveBinaryReader) delayedSwitchTexture() (delayedSwitchTextureSaveState, error) {
	var v delayedSwitchTextureSaveState
	var err error
	for _, dst := range []*int{&v.Line, &v.Sidedef, &v.Tics} {
		if *dst, err = r.int(); err != nil {
			return v, err
		}
	}
	if v.Top, err = r.str(); err != nil {
		return v, err
	}
	if v.Bottom, err = r.str(); err != nil {
		return v, err
	}
	if v.Mid, err = r.str(); err != nil {
		return v, err
	}
	return v, nil
}
func (r saveBinaryReader) doorThinkerMap() (map[int]doorThinkerSaveState, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make(map[int]doorThinkerSaveState, n)
	for i := uint32(0); i < n; i++ {
		key, err := r.int()
		if err != nil {
			return nil, err
		}
		var v doorThinkerSaveState
		for _, dst := range []*int{&v.Sector, &v.Type, &v.Direction, &v.TopWait, &v.TopCountdown} {
			if *dst, err = r.int(); err != nil {
				return nil, err
			}
		}
		if v.TopHeight, err = r.i64(); err != nil {
			return nil, err
		}
		if v.Speed, err = r.i64(); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}
func (r saveBinaryReader) floorThinkerMap() (map[int]floorThinkerSaveState, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make(map[int]floorThinkerSaveState, n)
	for i := uint32(0); i < n; i++ {
		key, err := r.int()
		if err != nil {
			return nil, err
		}
		var v floorThinkerSaveState
		if v.Sector, err = r.int(); err != nil {
			return nil, err
		}
		if v.Direction, err = r.int(); err != nil {
			return nil, err
		}
		if v.Speed, err = r.i64(); err != nil {
			return nil, err
		}
		if v.DestHeight, err = r.i64(); err != nil {
			return nil, err
		}
		if v.Finish, err = r.u8(); err != nil {
			return nil, err
		}
		if v.FinishFlat, err = r.str(); err != nil {
			return nil, err
		}
		if v.FinishSpecial, err = r.i16(); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}
func (r saveBinaryReader) platThinkerMap() (map[int]platThinkerSaveState, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make(map[int]platThinkerSaveState, n)
	for i := uint32(0); i < n; i++ {
		key, err := r.int()
		if err != nil {
			return nil, err
		}
		var v platThinkerSaveState
		if v.Sector, err = r.int(); err != nil {
			return nil, err
		}
		if v.Type, err = r.u8(); err != nil {
			return nil, err
		}
		if v.Status, err = r.u8(); err != nil {
			return nil, err
		}
		if v.OldStatus, err = r.u8(); err != nil {
			return nil, err
		}
		if v.Speed, err = r.i64(); err != nil {
			return nil, err
		}
		if v.Low, err = r.i64(); err != nil {
			return nil, err
		}
		if v.High, err = r.i64(); err != nil {
			return nil, err
		}
		if v.Wait, err = r.int(); err != nil {
			return nil, err
		}
		if v.Count, err = r.int(); err != nil {
			return nil, err
		}
		if v.FinishFlat, err = r.str(); err != nil {
			return nil, err
		}
		if v.FinishSpecial, err = r.i16(); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}
func (r saveBinaryReader) ceilingThinkerMap() (map[int]ceilingThinkerSaveState, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make(map[int]ceilingThinkerSaveState, n)
	for i := uint32(0); i < n; i++ {
		key, err := r.int()
		if err != nil {
			return nil, err
		}
		var v ceilingThinkerSaveState
		if v.Sector, err = r.int(); err != nil {
			return nil, err
		}
		if v.Direction, err = r.int(); err != nil {
			return nil, err
		}
		if v.OldDirection, err = r.int(); err != nil {
			return nil, err
		}
		action, err := r.str()
		if err != nil {
			return nil, err
		}
		v.Action = mapdata.CeilingAction(action)
		if v.Speed, err = r.i64(); err != nil {
			return nil, err
		}
		if v.TopHeight, err = r.i64(); err != nil {
			return nil, err
		}
		if v.BottomHeight, err = r.i64(); err != nil {
			return nil, err
		}
		if v.Crush, err = r.bool(); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}

func readSlice[T any](r saveBinaryReader, fn func() (T, error)) ([]T, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]T, n)
	for i := range out {
		if out[i], err = fn(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func readBoolSlice(r saveBinaryReader) ([]bool, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]bool, n)
	for i := range out {
		if out[i], err = r.bool(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func readIntSlice(r saveBinaryReader) ([]int, error) {
	return readSlice(r, r.int)
}
func readI64Slice(r saveBinaryReader) ([]int64, error) {
	return readSlice(r, r.i64)
}
func readU32Slice(r saveBinaryReader) ([]uint32, error) {
	return readSlice(r, r.u32)
}
func readU16Slice(r saveBinaryReader) ([]uint16, error) {
	return readSlice(r, r.u16)
}
func readU8Slice(r saveBinaryReader) ([]uint8, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	if err := r.read(out); err != nil {
		return nil, err
	}
	return out, nil
}
