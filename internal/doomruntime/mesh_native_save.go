package doomruntime

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gddoom/internal/doomrand"
	"gddoom/internal/music"
	"gddoom/internal/runtimecfg"
)

// NativeSaveSlot exposes the same slot metadata and storage used by Ebiten.
type NativeSaveSlot struct {
	Slot             int
	Description      string
	Current          string
	Health, WorldTic int
	ModTime          time.Time
	Present          bool
	WADSources       []runtimecfg.WADSource
}

func (c *NativeCampaign) SaveSlots(includeNew bool) []NativeSaveSlot {
	slots := c.session.availableSaveSlots(includeNew)
	out := make([]NativeSaveSlot, len(slots))
	for i, slot := range slots {
		out[i].Slot = slot
		if info, ok := c.session.readSaveSlotInfo(slot); ok {
			out[i] = NativeSaveSlot{Slot: slot, Description: info.Description, Current: string(info.Current), Health: info.Health, WorldTic: info.WorldTic, ModTime: info.ModTime, Present: true}
			for _, src := range info.WADSources {
				out[i].WADSources = append(out[i].WADSources, runtimecfg.WADSource{Name: src.Name, Hash: src.Hash})
			}
		}
	}
	return out
}

func (c *NativeCampaign) SaveData(description string) ([]byte, error) {
	if c.Phase() != NativeCampaignPlaying || c.DemoStatus().Active || c.FrontendStatus().Active {
		return nil, errSaveGameUnavailable
	}
	return c.session.marshalSaveGame(description)
}

func (c *NativeCampaign) SaveSlot(slot int) error {
	description := fmt.Sprintf("Slot %d", slot)
	if slot <= 0 {
		description = "Quicksave"
	}
	data, err := c.SaveData(description)
	if err != nil {
		return err
	}
	if err := writeSavedSlotData(slot, data); err != nil {
		return err
	}
	// Until the host captures this save's frame, an older preview is misleading.
	return deleteSavedSlotThumbnailData(slot)
}

func (c *NativeCampaign) LoadSlot(slot int) error {
	data, err := readSavedSlotData(slot)
	if errors.Is(err, os.ErrNotExist) {
		return errNoSavedGame
	}
	if err != nil {
		return err
	}
	return c.LoadData(data)
}

// LoadData restores the shared snapshot without initializing Ebiten's window or
// audio. Decoding and map loading finish before the live campaign is replaced.
func (c *NativeCampaign) LoadData(data []byte) error {
	sg := c.session
	file, err := sg.loadSnapshot(data, saveGameMagic, saveGameVersion)
	if err != nil {
		return err
	}
	if err := c.applyNativeSnapshot(file); err != nil {
		return err
	}
	return c.BroadcastMandatoryKeyframe()
}

func (c *NativeCampaign) CaptureKeyframe() ([]byte, error) {
	return c.session.marshalNetplayKeyframe()
}

func (c *NativeCampaign) LoadKeyframe(data []byte) error {
	file, err := c.session.loadSnapshot(data, keyframeMagic, keyframeVersion)
	if err != nil {
		return err
	}
	return c.applyNativeSnapshot(file)
}

func (c *NativeCampaign) applyNativeSnapshot(file saveFile) error {
	sg := c.session
	if sg.opts.NewGameLoader == nil {
		return fmt.Errorf("save load requires NewGameLoader")
	}
	if strings.TrimSpace(string(file.Current)) == "" {
		return fmt.Errorf("save missing current map")
	}
	loaded, err := sg.opts.NewGameLoader(string(file.Current))
	if err != nil {
		return fmt.Errorf("load saved map %s: %w", file.Current, err)
	}
	if loaded == nil || loaded.Name != file.Current {
		return fmt.Errorf("load saved map %s: invalid map", file.Current)
	}
	sg.freezeDemoRecord()
	c.CloseDemoTrace()
	opts := sg.opts
	opts.RecordDemoPath = ""
	opts.DemoScript, opts.DemoTracePath = nil, ""
	applySavedSessionOptions(&opts, file.Game.Session)
	opts.AlwaysRun = file.Game.AlwaysRun
	fresh := NewNativeMeshGame(loaded, opts)
	restoreGameSaveState(fresh.g, file.Game)
	fresh.SetGammaLevel(file.Game.GammaLevel)
	opts.InitialGammaLevel = fresh.GammaLevel()
	doomrand.SetState(file.RNG.MenuIndex, file.RNG.PlayIndex)
	sg.g.clearPendingSoundState()
	sg.opts, sg.g, sg.current = opts, fresh.g, file.Current
	sg.levelCarryover = nil
	sg.secretVisited = false // Matches the main host's current save format.
	sg.intermission = sessionIntermission{}
	sg.finale = sessionFinale{}
	sg.frontend = frontendState{}
	c.Game, c.complete = fresh, false
	c.lastNetworkKeyframe = 0
	lump, _ := music.MapLumpName(string(file.Current))
	c.queueMusic(lump)
	warning := compareSaveWADSources(file.WADSources, captureSaveWADSources(opts.WADSources))
	if warning != "" {
		fresh.Notify("WAD WARNING: " + warning)
	} else {
		fresh.Notify("GAME LOADED")
	}
	return nil
}

func NativeSaveThumbnail(slot int) ([]byte, time.Time, error) {
	return readSavedSlotThumbnailDataWithModTime(slot)
}
func WriteNativeSaveThumbnail(slot int, png []byte) error {
	return writeSavedSlotThumbnailData(slot, png)
}

func (c *NativeCampaign) SetControls(alwaysRun bool, mouseSpeed float64) {
	c.session.opts.AlwaysRun, c.session.opts.MouseLookSpeed = alwaysRun, mouseSpeed
	c.Game.g.opts.AlwaysRun, c.Game.g.opts.MouseLookSpeed = alwaysRun, mouseSpeed
	c.Game.g.alwaysRun = alwaysRun
}
func (n *NativeMeshGame) AlwaysRun() bool { return n.g.alwaysRun }
func (n *NativeMeshGame) SkillLevel() int { return n.g.opts.SkillLevel }
func (n *NativeMeshGame) MapActive() bool { return n.g.mode == viewMap }

func (n *NativeMeshGame) AutoWeaponSwitch() bool { return n.g.autoWeaponSwitch }
