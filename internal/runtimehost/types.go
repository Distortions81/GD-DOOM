package runtimehost

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

type Meta struct {
	Close               func()
	Err                 func() error
	EffectiveDemoRecord func() []demo.Tic
	Options             func() runtimecfg.Options
	StartMapName        func() mapdata.MapName
}
