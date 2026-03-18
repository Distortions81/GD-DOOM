package doomtex

import "github.com/hajimehoshi/ebiten/v2"

func (s *Set) BuildTextureEbiten(name string, palette int) (*ebiten.Image, error) {
	rgba, w, h, err := s.BuildTextureRGBA(name, palette)
	if err != nil {
		return nil, err
	}
	img := ebiten.NewImage(w, h)
	img.WritePixels(rgba)
	return img, nil
}
