package doomsession

import (
	"fmt"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/automap"
	"gddoom/internal/session"

	"github.com/hajimehoshi/ebiten/v2"
)

type NextMapFunc = automap.NextMapFunc
type Options = automap.Options
type RuntimeSettings = automap.RuntimeSettings
type DemoTic = automap.DemoTic

type Session struct {
	inner *automap.Session
}

func Run(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	sess := New(m, opts, nextMap)
	defer sess.Close()
	if err := session.Run(session.New(sess)); err != nil {
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sess.Options().RecordDemoPath; p != "" {
		rec := sess.EffectiveDemoRecord()
		demo, derr := automap.BuildRecordedDemo(sess.StartMapName(), sess.Options(), rec)
		if derr != nil {
			return fmt.Errorf("build demo recording: %w", derr)
		}
		if werr := automap.SaveDemoScript(p, demo); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(rec))
	}
	return sess.Err()
}

func New(m *mapdata.Map, opts Options, nextMap NextMapFunc) *Session {
	return &Session{inner: automap.NewSession(m, opts, nextMap)}
}

func (s *Session) Update() error {
	if s == nil || s.inner == nil {
		return ebiten.Termination
	}
	return s.inner.Update()
}

func (s *Session) Draw(screen *ebiten.Image) {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Draw(screen)
}

func (s *Session) Layout(outsideWidth, outsideHeight int) (int, int) {
	if s == nil || s.inner == nil {
		if outsideWidth < 1 {
			outsideWidth = 1
		}
		if outsideHeight < 1 {
			outsideHeight = 1
		}
		return outsideWidth, outsideHeight
	}
	return s.inner.Layout(outsideWidth, outsideHeight)
}

func (s *Session) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.DrawFinalScreen(screen, offscreen, geoM)
}

func (s *Session) Close() {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Close()
}

func (s *Session) Err() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Err()
}

func (s *Session) EffectiveDemoRecord() []DemoTic {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.EffectiveDemoRecord()
}

func (s *Session) Options() Options {
	if s == nil || s.inner == nil {
		return Options{}
	}
	return s.inner.Options()
}

func (s *Session) StartMapName() mapdata.MapName {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.StartMapName()
}
