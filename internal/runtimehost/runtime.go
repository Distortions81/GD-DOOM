package runtimehost

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/session"
)

type Accessors struct {
	Close               func()
	Err                 func() error
	EffectiveDemoRecord func() []demo.Tic
	Options             func() runtimecfg.Options
	StartMapName        func() mapdata.MapName
}

func NewGame(runtime session.Runtime, accessors Accessors) (*session.Game, Meta) {
	game := session.New(runtime)
	return game, Meta{
		Close:               accessors.Close,
		Err:                 accessors.Err,
		EffectiveDemoRecord: accessors.EffectiveDemoRecord,
		Options:             accessors.Options,
		StartMapName:        accessors.StartMapName,
	}
}
