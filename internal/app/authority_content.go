package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"slices"

	"gddoom/internal/audiofx"
	"gddoom/internal/lobby"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

// Preparation owns an isolated, verified stack. The live game is untouched
// until Load builds a complete replacement on the host thread.
func prepareAuthorityContent(ctx context.Context, address string, room lobby.Room, paths, hashes []string, progress func(runtimecfg.AuthorityContentProgress), restoreHostCallbacks func(*runtimecfg.Options)) (runtimecfg.AuthorityContentPreparation, error) {
	var empty runtimecfg.AuthorityContentPreparation
	state, err := lobby.Fetch(ctx, address)
	if err != nil {
		return empty, err
	}
	var pack lobby.Pack
	for _, candidate := range state.Packs {
		if candidate.ID == room.Settings.PackID {
			pack = candidate
			break
		}
	}
	if !slices.Equal(pack.WADHashes, room.Manifest.WADHashes) {
		return empty, fmt.Errorf("room WADs changed; refresh the game list")
	}
	settings := room.Settings
	settings.Map = room.Manifest.Map // A running room may have advanced levels.
	manifest, err := lobby.ValidateSettings(settings, pack)
	wantKey, wantErr := manifest.Key()
	roomKey, roomErr := room.Manifest.Key()
	if err != nil || wantErr != nil || roomErr != nil || wantKey != roomKey {
		return empty, fmt.Errorf("room settings changed; refresh the game list")
	}
	if err := lobby.ValidateDownloadPack(pack); err != nil {
		return empty, err
	}
	data, err := resolveAuthorityContent(ctx, address, pack, paths, hashes, progress)
	if err != nil {
		return empty, err
	}
	names := make([]string, len(pack.Files))
	for i, file := range pack.Files {
		names[i] = file.Name
	}
	contentPaths, cleanup, err := wad.RegisterMemoryFiles(names, data)
	if err != nil {
		return empty, err
	}
	return runtimecfg.AuthorityContentPreparation{
		Cancel: cleanup,
		Load: func(current runtimecfg.Options) (runtimecfg.AuthorityContentBundle, error) {
			cfg := contentRenderConfig(current)
			cfg.authorityLobbyURL = address
			cfg.selectedMap, cfg.mapExplicit = room.Manifest.Map, true
			cfg.pwadPaths = contentPaths[1:]
			bundle, err := buildRenderBundle(contentPaths[0], cfg, io.Discard)
			if err != nil {
				cleanup()
				return runtimecfg.AuthorityContentBundle{}, err
			}
			bundle.opts.AuthorityContentCleanup = cleanup
			preserveAuthorityContentPreferences(&bundle.opts, current)
			if restoreHostCallbacks != nil {
				restoreHostCallbacks(&bundle.opts)
			}
			// Rebind callbacks after restoring host persistence so subsequent
			// content switches retain it without retaining a retired runtime.
			if err := configureAuthorityLobby(&bundle.opts, contentPaths, address); err != nil {
				if bundle.opts.SharedPCSpeaker != nil {
					bundle.opts.SharedPCSpeaker.Close()
				}
				cleanup()
				return runtimecfg.AuthorityContentBundle{}, err
			}
			return runtimecfg.AuthorityContentBundle{Map: bundle.m, Options: bundle.opts, NextMap: bundle.nextMap}, nil
		},
	}, nil
}

func preserveAuthorityContentPreferences(next *runtimecfg.Options, current runtimecfg.Options) {
	next.AuthorityServers = slices.Clone(current.AuthorityServers)
	next.RespawnMonsters = current.RespawnMonsters
	next.SmoothCameraYaw = current.SmoothCameraYaw
	next.ZombiemanThinkerBlend, next.DebugMonsterThinkerBlend = current.ZombiemanThinkerBlend, current.DebugMonsterThinkerBlend
	next.DisableMaskedMidFastPaths, next.DisableGeometryAspectCorrect = current.DisableMaskedMidFastPaths, current.DisableGeometryAspectCorrect
	next.VoiceCodec, next.VoiceG726BitsPerSample = current.VoiceCodec, current.VoiceG726BitsPerSample
	next.VoiceBitrate, next.VoiceSampleRate = current.VoiceBitrate, current.VoiceSampleRate
	next.VoiceAGCEnabled, next.VoicePushToTalkEnabled, next.VoiceGateEnabled = current.VoiceAGCEnabled, current.VoicePushToTalkEnabled, current.VoiceGateEnabled
	next.VoiceGateThreshold, next.VoiceInputDevice = current.VoiceGateThreshold, current.VoiceInputDevice
	next.VoiceInputLevel, next.VoiceInputGateActive = current.VoiceInputLevel, current.VoiceInputGateActive
	next.VoiceBandwidthMeter, next.VoiceSyncMeter = current.VoiceBandwidthMeter, current.VoiceSyncMeter
}

func resolveAuthorityContent(ctx context.Context, address string, pack lobby.Pack, paths, hashes []string, progress func(runtimecfg.AuthorityContentProgress)) ([][]byte, error) {
	if err := lobby.ValidateDownloadPack(pack); err != nil {
		return nil, err
	}
	local := make(map[string]string, len(hashes))
	for i, hash := range hashes {
		if i < len(paths) {
			local[hash] = paths[i]
		}
	}
	// Refuse the entire operation before making any download when a required
	// private file is absent. Distribution permission belongs to each file.
	var total int64
	for _, file := range pack.Files {
		if _, found := local[file.SHA256]; found {
			continue
		}
		if !file.Downloadable {
			return nil, fmt.Errorf("load %s locally first; this file is not available for download", file.Name)
		}
		total += file.Size
	}
	data := make([][]byte, len(pack.Files))
	var received int64
	for i, file := range pack.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var err error
		if path, found := local[file.SHA256]; found {
			data[i], err = readAuthorityWAD(path, file)
		} else {
			if progress != nil {
				progress(runtimecfg.AuthorityContentProgress{Stage: "DOWNLOADING", Name: file.Name, Received: received, Total: total})
			}
			data[i], err = lobby.Download(ctx, address, file)
			received += file.Size
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Name, err)
		}
		if _, err := wad.OpenData(file.Name, data[i]); err != nil {
			return nil, fmt.Errorf("%s is not a valid WAD: %w", file.Name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(runtimecfg.AuthorityContentProgress{Stage: "VERIFIED", Received: received, Total: total})
	}
	return data, nil
}

func readAuthorityWAD(path string, file lobby.PackFile) ([]byte, error) {
	data, found := wad.EmbeddedDataForPath(path)
	if !found {
		reader, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		data, err = io.ReadAll(io.LimitReader(reader, file.Size+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if int64(len(data)) != file.Size || fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
		return nil, fmt.Errorf("local WAD changed; reload the matching file first")
	}
	return data, nil
}

// Rebuild from the live presentation/input preferences while importing every
// content bank and map/music loader from the replacement stack.
func contentRenderConfig(o runtimecfg.Options) renderBuildConfig {
	cfg := renderBuildConfig{
		authorityJoinDefaults: o.AuthorityJoinDefaults,
		width:                 o.Width, height: o.Height, zoom: o.StartZoom,
		detailLevel: o.InitialDetailLevel, detailLevelExplicit: true, autoDetail: o.AutoDetail, gammaLevel: o.InitialGammaLevel,
		debug: o.Debug, debugEvents: o.DebugEvents, playerSlot: o.PlayerSlot,
		skillLevel: o.SkillLevel, gameMode: o.GameMode, showNoSkillItems: o.ShowNoSkillItems, showAllItems: o.ShowAllItems, noMonsters: o.NoMonsters,
		mouseLook: o.MouseLook, mouseInvert: o.MouseInvert, mouseLookSpeed: o.MouseLookSpeed, keyboardTurnSpeed: o.KeyboardTurnSpeed,
		musicVolume: o.MusicVolume, musPanMax: o.MUSPanMax, musVolumeCompression: o.MUSVolumeCompression, oplVolume: o.OPLVolume,
		audioPreEmphasis: o.AudioPreEmphasis, musicBackend: o.MusicBackend, soundFontPath: o.MusicSoundFontPath,
		sfxVolume: o.SFXVolume, pcSpeakerVolume: o.PCSpeakerVolume, sfxPitchShift: o.SFXPitchShift,
		fastMonsters: o.FastMonsters, alwaysRun: o.AlwaysRun, autoWeaponSwitch: o.AutoWeaponSwitch, cheatLevel: o.CheatLevel, invuln: o.Invulnerable,
		sourcePortMode: o.SourcePortMode, sourcePortThingRenderMode: o.SourcePortThingRenderMode, sourcePortThingBlendFrames: o.SourcePortThingBlendFrames,
		sourcePortSectorLighting: o.SourcePortSectorLighting, doomLighting: !o.DisableDoomLighting, kageShader: o.KageShader, crtEffect: o.CRTEffect,
		wallOcclusion: !o.DisableWallOcclusion, wallSpanReject: !o.DisableWallSpanReject, wallSpanClip: !o.DisableWallSpanClip, wallSliceOcclusion: !o.DisableWallSliceOcclusion,
		billboardClipping: !o.DisableBillboardClipping, gpuRenderer: o.GPURenderer, meshRenderer: o.MeshRenderer, rendererWorkers: o.RendererWorkers,
		textureAnimCrossfadeFrames: o.TextureAnimCrossfadeFrames, noVsync: o.NoVsync, noFPS: o.NoFPS, showTPS: o.ShowTPS, noAspectCorrection: o.DisableAspectCorrection,
		allCheats: o.AllCheats, importTextures: true, importPCSpeaker: len(o.PCSpeakerBank) > 0, pcSpeaker: len(o.PCSpeakerBank) > 0,
		inputBindings: o.InputBindings,
	}
	switch o.PCSpeakerVariant {
	case audiofx.PCSpeakerVariantSmallSpeaker:
		cfg.pcSpeakerVariant = "small-speaker"
	case audiofx.PCSpeakerVariantPiezo:
		cfg.pcSpeakerVariant = "piezo"
	default:
		cfg.pcSpeakerVariant = "clean"
	}
	return cfg
}
