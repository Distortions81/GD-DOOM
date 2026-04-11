package app

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/music"
	"gddoom/internal/platformcfg"
)

func TestResolveIWADAliasPathResolvesRequestedPathCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	actual := filepath.Join(td, "doom1.wad")
	if err := os.WriteFile(actual, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	got := resolveIWADAliasPath(filepath.Join(td, "DOOM1.WAD"))
	if got != actual {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, actual)
	}
}

func TestResolveIWADAliasPathFallsBackToDoomAliasCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	alias := filepath.Join(td, "doom.wad")
	if err := os.WriteFile(alias, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write alias wad: %v", err)
	}
	missing := filepath.Join(td, "DOOM1.WAD")
	got := resolveIWADAliasPath(missing)
	if got != alias {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, alias)
	}
}

func TestResolveIWADAliasPathFallsBackToDoomUAliasCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	alias := filepath.Join(td, "doomu.wad")
	if err := os.WriteFile(alias, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write alias wad: %v", err)
	}
	missing := filepath.Join(td, "DOOM1.WAD")
	got := resolveIWADAliasPath(missing)
	if got != alias {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, alias)
	}
}

func TestResolveIWADAliasPathMapsDoomToDoomUCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	alias := filepath.Join(td, "doomu.wad")
	if err := os.WriteFile(alias, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write alias wad: %v", err)
	}
	missing := filepath.Join(td, "DOOM.WAD")
	got := resolveIWADAliasPath(missing)
	if got != alias {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, alias)
	}
}

func TestResolveIWADAliasPathResolvesNonDoomPathCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	actual := filepath.Join(td, "custom.wad")
	if err := os.WriteFile(actual, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write custom wad: %v", err)
	}
	got := resolveIWADAliasPath(filepath.Join(td, "CUSTOM.WAD"))
	if got != actual {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, actual)
	}
}

func TestResolveIWADAliasPathReturnsOriginalWhenNoCandidateExists(t *testing.T) {
	td := t.TempDir()
	missing := filepath.Join(td, "DOOM1.WAD")
	got := resolveIWADAliasPath(missing)
	if got != missing {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, missing)
	}
}

func TestDetectAvailableIWADChoicesSkipsSharewareUntilLast(t *testing.T) {
	td := t.TempDir()
	for _, name := range []string{"doom1.wad", "doomu.wad", "doom2.wad", "tnt.wad", "plutonia.wad"} {
		if err := os.WriteFile(filepath.Join(td, name), []byte("wad"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	choices := detectAvailableIWADChoices(td)
	want := []string{"doomu.wad", "doom2.wad", "tnt.wad", "plutonia.wad", "doom1.wad"}
	if len(choices) != len(want) {
		t.Fatalf("choices len=%d want=%d", len(choices), len(want))
	}
	for i, choice := range choices {
		if got := filepath.Base(choice.Path); got != want[i] {
			t.Fatalf("choice %d base=%q want=%q", i, got, want[i])
		}
	}
}

func TestDetectAvailableIWADChoicesDeduplicatesDoomAndDoomUAliases(t *testing.T) {
	td := t.TempDir()
	for _, name := range []string{"doom.wad", "doomu.wad"} {
		if err := os.WriteFile(filepath.Join(td, name), []byte("wad"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	choices := detectAvailableIWADChoices(td)
	if len(choices) != 1 {
		t.Fatalf("choices len=%d want=1", len(choices))
	}
	if got := filepath.Base(choices[0].Path); got != "doomu.wad" {
		t.Fatalf("choice base=%q want=doomu.wad", got)
	}
	if got := choices[0].Label; got != "The Ultimate DOOM" {
		t.Fatalf("choice label=%q want %q", got, "The Ultimate DOOM")
	}
}

func TestDetectAvailableIWADChoicesKeepsSharewareWhenOnlyOption(t *testing.T) {
	td := t.TempDir()
	if err := os.WriteFile(filepath.Join(td, "doom1.wad"), []byte("wad"), 0o644); err != nil {
		t.Fatalf("write doom1.wad: %v", err)
	}

	choices := detectAvailableIWADChoices(td)
	if len(choices) != 1 {
		t.Fatalf("choices len=%d want=1", len(choices))
	}
	if got := filepath.Base(choices[0].Path); got != "doom1.wad" {
		t.Fatalf("choice base=%q want=doom1.wad", got)
	}
}

func TestPickerDefaultsPreferSourcePortAndFirstNonSharewareIWAD(t *testing.T) {
	game, err := newIWADPickerGame([]iwadChoice{
		{Path: "/tmp/doom.wad", Label: "The Ultimate DOOM"},
		{Path: "/tmp/doom2.wad", Label: "DOOM II: Hell on Earth"},
	}, music.BackendImpSynth, "paper-speaker", nil)
	if err != nil {
		t.Fatalf("newIWADPickerGame() error: %v", err)
	}

	if game.profile != pickerProfileSourcePort {
		t.Fatalf("default profile=%v want sourceport", game.profile)
	}
	if game.synth != 0 {
		t.Fatalf("default synth=%d want=0", game.synth)
	}
	if game.selected != 0 {
		t.Fatalf("default selected=%d want=0", game.selected)
	}
}

func TestPickerAssetWADPathFallsBackToNonSharewareChoice(t *testing.T) {
	choices := []iwadChoice{
		{Path: "/tmp/plutonia.wad", Label: "Final DOOM: Plutonia"},
		{Path: "/tmp/custom-total-conversion.wad", Label: "Custom"},
	}

	if got := pickerAssetWADPath(choices); got != "/tmp/plutonia.wad" {
		t.Fatalf("pickerAssetWADPath() = %q want %q", got, "/tmp/plutonia.wad")
	}
}

func TestShouldOpenIWADPickerForWASMEvenWithExplicitWAD(t *testing.T) {
	if !shouldOpenIWADPicker(true, false, true, 1) {
		t.Fatal("WASM should force the IWAD/profile picker when a choice exists")
	}
}

func TestShouldOpenIWADPickerRequiresChoicesAndRender(t *testing.T) {
	if shouldOpenIWADPicker(false, true, true, 1) {
		t.Fatal("picker should stay closed when render is disabled")
	}
	if shouldOpenIWADPicker(true, true, true, 0) {
		t.Fatal("picker should stay closed when no choices exist")
	}
}

func TestWASMPickerStartsAtIWADStageEvenWithSingleChoice(t *testing.T) {
	game, err := newIWADPickerGame([]iwadChoice{
		{Path: "/tmp/doom1.wad", Label: "DOOM Shareware"},
	}, music.BackendImpSynth, "paper-speaker", nil)
	if err != nil {
		t.Fatalf("newIWADPickerGame() error: %v", err)
	}

	if got := game.stage; got != pickerStageProfile {
		t.Fatalf("default single-choice stage=%v want profile", got)
	}

	game.stage = pickerStageIWAD
	if got := game.stage; got != pickerStageIWAD {
		t.Fatalf("forced single-choice stage=%v want iwad", got)
	}
}

func TestPickerTouchControlsVisibleByDefaultInWASMMode(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	game := &iwadPickerGame{}
	if !game.shouldDrawPickerTouchControls() {
		t.Fatal("shouldDrawPickerTouchControls() = false, want true in wasm mode")
	}
}

func TestPickerBackMovesToPreviousStage(t *testing.T) {
	game := &iwadPickerGame{stage: pickerStageSynth, confirmArmed: true}

	if err := game.pickerBack(); err != nil {
		t.Fatalf("pickerBack() error = %v", err)
	}
	if game.stage != pickerStageSFX {
		t.Fatalf("stage=%v want=%v", game.stage, pickerStageSFX)
	}
	if game.confirmArmed {
		t.Fatal("confirmArmed=true want false")
	}
}

func TestPickerTouchButtonsPlaceEnterOnBottomRight(t *testing.T) {
	game := &iwadPickerGame{}
	buttons := game.pickerTouchButtons(320, 200)
	if len(buttons) != 4 {
		t.Fatalf("buttons len=%d want=4", len(buttons))
	}
	enter := buttons[3]
	if enter.label != "ENTER" {
		t.Fatalf("enter label=%q want ENTER", enter.label)
	}
	if enter.x <= 200 {
		t.Fatalf("enter.x=%v want > 200", enter.x)
	}
	if enter.y <= 120 {
		t.Fatalf("enter.y=%v want > 120", enter.y)
	}
}

func TestPickerTouchButtonsUseLogicalPickerCoordinates(t *testing.T) {
	game := &iwadPickerGame{}
	buttons := game.pickerTouchButtons(320, 200)
	if len(buttons) != 4 {
		t.Fatalf("buttons len=%d want=4", len(buttons))
	}
	enter := buttons[3]
	x := int(enter.x + enter.w/2)
	y := int(enter.y + enter.h/2)
	if !pickerTouchButtonContains(enter, float64(x), float64(y)) {
		t.Fatalf("logical picker touch %d,%d should hit ENTER button", x, y)
	}
}

func TestPickerDefaultsSynthFromInitialBackend(t *testing.T) {
	game, err := newIWADPickerGame([]iwadChoice{
		{Path: "/tmp/doom1.wad", Label: "DOOM Shareware"},
	}, music.BackendMeltySynth, "paper-speaker", nil)
	if err != nil {
		t.Fatalf("newIWADPickerGame() error: %v", err)
	}
	if game.synth != 2 {
		t.Fatalf("default synth=%d want=2", game.synth)
	}
}

func TestApplyPickerSynthSetsMeltySynthBackendAndPreservesVolume(t *testing.T) {
	cfg := renderBuildConfig{
		musicBackend: music.BackendImpSynth,
		musicVolume:  1.0,
	}

	got := applyPickerSynth(cfg, 2)

	if got.musicBackend != music.BackendMeltySynth {
		t.Fatalf("backend=%q want %q", got.musicBackend, music.BackendMeltySynth)
	}
	if got.musicVolume != 1.0 {
		t.Fatalf("musicVolume=%v want 1.0", got.musicVolume)
	}
}

func TestApplyPickerSynthKeepsSoundFontWhenAlreadySet(t *testing.T) {
	cfg := renderBuildConfig{
		musicBackend:  music.BackendImpSynth,
		musicVolume:   1.0,
		soundFontPath: "soundfonts/SC55-HQ.sf2",
	}

	got := applyPickerSynth(cfg, 3)

	if got.musicBackend != music.BackendMeltySynth {
		t.Fatalf("backend=%q want %q", got.musicBackend, music.BackendMeltySynth)
	}
	if got.musicVolume != 1.0 {
		t.Fatalf("musicVolume=%v want 1.0", got.musicVolume)
	}
	if got.soundFontPath != "soundfonts/SC55-HQ.sf2" {
		t.Fatalf("soundFontPath=%q want %q", got.soundFontPath, "soundfonts/SC55-HQ.sf2")
	}
}

func TestApplyPickerSynthMeltySynthSetsHQSoundFontPath(t *testing.T) {
	cfg := renderBuildConfig{
		musicBackend: music.BackendImpSynth,
		musicVolume:  1.0,
	}

	got := applyPickerSynth(cfg, 3)

	if got.musicBackend != music.BackendMeltySynth {
		t.Fatalf("backend=%q want %q", got.musicBackend, music.BackendMeltySynth)
	}
	if got.musicVolume != 1.0 {
		t.Fatalf("musicVolume=%v want 1.0", got.musicVolume)
	}
	if got.soundFontPath != "soundfonts/SC55-HQ.sf2" {
		t.Fatalf("soundFontPath=%q want %q", got.soundFontPath, "soundfonts/SC55-HQ.sf2")
	}
}

func TestApplyPickerSynthSGMHQSetsSoundFontPath(t *testing.T) {
	cfg := renderBuildConfig{
		musicBackend: music.BackendImpSynth,
		musicVolume:  1.0,
	}

	got := applyPickerSynth(cfg, 4)

	if got.musicBackend != music.BackendMeltySynth {
		t.Fatalf("backend=%q want %q", got.musicBackend, music.BackendMeltySynth)
	}
	if got.musicVolume != 1.0 {
		t.Fatalf("musicVolume=%v want 1.0", got.musicVolume)
	}
	if got.soundFontPath != music.BrowserSGMHQSoundFontPath() {
		t.Fatalf("soundFontPath=%q want %q", got.soundFontPath, music.BrowserSGMHQSoundFontPath())
	}
}

func TestApplyPickerSynthGeneralMIDISetsSoundFontPath(t *testing.T) {
	cfg := renderBuildConfig{
		musicBackend: music.BackendImpSynth,
		musicVolume:  1.0,
	}

	got := applyPickerSynth(cfg, 2)

	if got.musicBackend != music.BackendMeltySynth {
		t.Fatalf("backend=%q want %q", got.musicBackend, music.BackendMeltySynth)
	}
	if got.musicVolume != 1.0 {
		t.Fatalf("musicVolume=%v want 1.0", got.musicVolume)
	}
	if got.soundFontPath != "soundfonts/general-midi.sf2" {
		t.Fatalf("soundFontPath=%q want %q", got.soundFontPath, "soundfonts/general-midi.sf2")
	}
}

func TestSoundFontDefaultRankPrefersSC55(t *testing.T) {
	if got := soundFontDefaultRank("soundfonts/sc55.sf2"); got != 0 {
		t.Fatalf("rank(sc55)=%d want 0", got)
	}
	if got := soundFontDefaultRank("SGM-HQ.sf2"); got != 1 {
		t.Fatalf("rank(sgm-hq)=%d want 1", got)
	}
	if got := soundFontDefaultRank("soundfonts/general-midi.sf2"); got != 2 {
		t.Fatalf("rank(general-midi)=%d want 2", got)
	}
}
