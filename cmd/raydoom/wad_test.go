//go:build raylib && cgo && !js

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

type nativeTestLump struct {
	name string
	data []byte
}

func writeNativeTestWAD(t *testing.T, path string, lumps []nativeTestLump) {
	t.Helper()
	data := make([]byte, 12)
	copy(data, "PWAD")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(lumps)))
	dir := make([]byte, 16*len(lumps))
	for i, lump := range lumps {
		binary.LittleEndian.PutUint32(dir[i*16:i*16+4], uint32(len(data)))
		binary.LittleEndian.PutUint32(dir[i*16+4:i*16+8], uint32(len(lump.data)))
		copy(dir[i*16+8:i*16+16], lump.name)
		data = append(data, lump.data...)
	}
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)))
	data = append(data, dir...)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// Make a real E1M3 replacement plus independently overridden flat, status bar,
// sprite, font, sound and music. The second overlay changes the bar again.
func nativeWADOverlayFixture(t *testing.T) (string, string, string) {
	t.Helper()
	base := "../../DOOM1.WAD"
	wf, err := wad.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M3")
	if err != nil {
		t.Fatal(err)
	}
	marker, ok := wf.LumpByName("E1M3")
	if !ok {
		t.Fatal("missing E1M3")
	}
	var lumps []nativeTestLump
	for _, lump := range wf.Lumps[marker.Index : marker.Index+11] {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatal(err)
		}
		if lump.Name == "SECTORS" {
			binary.LittleEndian.PutUint16(data[20:22], 173)
		}
		lumps = append(lumps, nativeTestLump{lump.Name, data})
	}
	recolor := func(name string, index byte) nativeTestLump {
		lump, ok := wf.LumpByName(name)
		if !ok {
			t.Fatal("missing patch", name)
		}
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatal(err)
		}
		width := int(binary.LittleEndian.Uint16(data[:2]))
		for x := range width {
			pos := int(binary.LittleEndian.Uint32(data[8+4*x : 12+4*x]))
			for data[pos] != 255 {
				height := int(data[pos+1])
				for i := range height {
					data[pos+3+i] = index
				}
				pos += height + 4
			}
		}
		return nativeTestLump{name, data}
	}
	for _, name := range []string{"STBAR", "TROOA1", "STCFN065"} {
		lumps = append(lumps, recolor(name, 96))
	}
	lumps = append(lumps, nativeTestLump{"F_START", nil}, nativeTestLump{m.Sectors[0].FloorPic, bytes.Repeat([]byte{160}, 4096)}, nativeTestLump{"F_END", nil})
	for target, source := range map[string]string{"D_E1M3": "D_E1M1", "DSPISTOL": "DSSHOTGN"} {
		lump, ok := wf.LumpByName(source)
		if !ok {
			t.Fatal("missing media", source)
		}
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatal(err)
		}
		lumps = append(lumps, nativeTestLump{target, data})
	}
	dir := t.TempDir()
	first, last := filepath.Join(dir, "level.wad"), filepath.Join(dir, "bar.wad")
	writeNativeTestWAD(t, first, lumps)
	writeNativeTestWAD(t, last, []nativeTestLump{recolor("STBAR", 160)})
	return base, first, last
}

func TestNativeWADStackReplacesGeometryAllAssetBanksAndSaveSources(t *testing.T) {
	base, first, last := nativeWADOverlayFixture(t)
	wf, paths, err := launchcatalog.OpenWADStack(base, []string{first, last})
	if err != nil {
		t.Fatal(err)
	}
	name, err := launchcatalog.DefaultStartMap(wf, paths[1:])
	if err != nil || name != "E1M3" {
		t.Fatal("art overlay hid the custom map", name, err)
	}
	m, err := mapdata.LoadMap(wf, name)
	if err != nil {
		t.Fatal(err)
	}
	if m.Sectors[0].Light != 173 {
		t.Fatal("native map used base sectors")
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	for name, texture := range map[string]doomruntime.WallTexture{"STBAR": opts.StatusPatchBank["STBAR"], "TROOA1": opts.SpritePatchBank["TROOA1"], "STCFN065": opts.MessageFontBank['A']} {
		want := byte(96)
		if name == "STBAR" {
			want = 160
		}
		count := 0
		for i := range texture.Indexed {
			if texture.RGBA[i*4+3] != 0 {
				count++
				if texture.Indexed[i] != want {
					t.Fatalf("%s retained base pixels", name)
				}
			}
		}
		if count == 0 {
			t.Fatal("missing override patch", name)
		}
	}
	flat := opts.FlatBankIndexed[m.Sectors[0].FloorPic]
	if !bytes.Equal(flat, bytes.Repeat([]byte{160}, 4096)) {
		t.Fatal("flat namespace did not use overlay")
	}
	baseFile, err := wad.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	for target, source := range map[string]string{"D_E1M3": "D_E1M1", "DSPISTOL": "DSSHOTGN"} {
		a, _ := wf.LumpByName(target)
		b, _ := baseFile.LumpByName(source)
		actual, err := wf.LumpData(a)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := baseFile.LumpData(b)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatal("media retained base lump", target)
		}
	}
	opts.WADSources, opts.WADHash = launchcatalog.BuildWADSources(paths), launchcatalog.HashWADStackSHA1(paths)
	opts.NewGameLoader = func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }
	c := doomruntime.NewNativeCampaign(doomruntime.NewNativeMeshGame(m, opts), opts, nil)
	data, err := c.SaveData("overlaid map")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadData(data); err != nil {
		t.Fatal(err)
	}
	if len(opts.WADSources) != 3 || opts.WADSources[1].Name != "level.wad" || opts.WADSources[2].Name != "bar.wad" || c.Map().Sectors[0].Light != 173 || c.Game.Frame(1).Message != "GAME LOADED" {
		t.Fatal("matching stack did not survive save/load")
	}
	opts.WADSources = launchcatalog.BuildWADSources([]string{base})
	other := doomruntime.NewNativeCampaign(doomruntime.NewNativeMeshGame(m, opts), opts, nil)
	if err := other.LoadData(data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(other.Game.Frame(1).Message), []byte("WAD WARNING")) {
		t.Fatal("omitting PWADs did not warn on loading the save")
	}
}
