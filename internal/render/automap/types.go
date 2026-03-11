package automap

import (
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/media"
	"gddoom/internal/runtimecfg"
)

type RuntimeSettings = gameplay.RuntimeSettings

type Options = runtimecfg.Options

type WallTexture = media.WallTexture

type RunResult struct {
	LevelExited bool
	SecretExit  bool
}

type PCMSample = media.PCMSample

type SoundBank = media.SoundBank

type DemoTic = demo.Tic

type DemoHeader = demo.Header

type DemoScript = demo.Script
