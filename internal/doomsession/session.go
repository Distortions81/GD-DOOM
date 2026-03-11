package doomsession

import (
	"fmt"
	"strings"

	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/automap"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/runtimehost"
	"gddoom/internal/session"

	"github.com/hajimehoshi/ebiten/v2"
)

type NextMapFunc = runtimehost.NextMapFunc
type Options = runtimecfg.Options
type RuntimeSettings = gameplay.RuntimeSettings
type DemoTic = demo.Tic

type Session struct {
	game *session.Game
	meta runtimehost.Meta
}

func Run(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	sess := New(m, opts, nextMap)
	defer sess.Close()
	if err := session.Run(session.New(sess)); err != nil {
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sess.Options().RecordDemoPath; p != "" {
		rec := sess.EffectiveDemoRecord()
		opts := sess.Options()
		skill := opts.SkillLevel - 1
		if skill < 0 {
			skill = 0
		}
		demoRec, derr := demo.BuildRecorded(sess.StartMapName(), demo.RecordingOptions{
			Skill:        skill,
			Deathmatch:   strings.EqualFold(opts.GameMode, "deathmatch"),
			FastMonsters: opts.FastMonsters,
		}, rec)
		if derr != nil {
			return fmt.Errorf("build demo recording: %w", derr)
		}
		if werr := demo.Save(p, demoRec); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(rec))
	}
	return sess.Err()
}

func New(m *mapdata.Map, opts Options, nextMap NextMapFunc) *Session {
	opts, windowW, windowH := runtimecfg.NormalizeRunDimensions(opts)
	game, meta := automap.NewRuntime(m, opts, nextMap)
	runtimehost.ConfigureInitialHost(opts, windowW, windowH, m.Name)
	return &Session{game: game, meta: meta}
}

func (s *Session) Update() error {
	if s == nil || s.game == nil {
		return ebiten.Termination
	}
	return s.game.Update()
}

func (s *Session) Draw(screen *ebiten.Image) {
	if s == nil || s.game == nil {
		return
	}
	s.game.Draw(screen)
}

func (s *Session) Layout(outsideWidth, outsideHeight int) (int, int) {
	if s == nil || s.game == nil {
		if outsideWidth < 1 {
			outsideWidth = 1
		}
		if outsideHeight < 1 {
			outsideHeight = 1
		}
		return outsideWidth, outsideHeight
	}
	return s.game.Layout(outsideWidth, outsideHeight)
}

func (s *Session) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if s == nil || s.game == nil {
		return
	}
	s.game.DrawFinalScreen(screen, offscreen, geoM)
}

func (s *Session) Close() {
	if s == nil || s.meta.Close == nil {
		return
	}
	s.meta.Close()
}

func (s *Session) Err() error {
	if s == nil || s.meta.Err == nil {
		return nil
	}
	return s.meta.Err()
}

func (s *Session) EffectiveDemoRecord() []DemoTic {
	if s == nil || s.meta.EffectiveDemoRecord == nil {
		return nil
	}
	return s.meta.EffectiveDemoRecord()
}

func (s *Session) Options() Options {
	if s == nil || s.meta.Options == nil {
		return Options{}
	}
	return s.meta.Options()
}

func (s *Session) StartMapName() mapdata.MapName {
	if s == nil || s.meta.StartMapName == nil {
		return ""
	}
	return s.meta.StartMapName()
}
