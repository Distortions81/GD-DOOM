module gddoom

go 1.26.6

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/Distortions81/g726 v0.0.8
	github.com/Distortions81/impsynth v0.1.3
	github.com/dustin/go-humanize v1.0.1
	github.com/hajimehoshi/ebiten/v2 v2.10.4
	github.com/klauspost/compress v1.18.6
	github.com/remeh/sizedwaitgroup v1.0.0
	github.com/sinshu/go-meltysynth v0.1.2
	github.com/youthlin/silk v0.0.4
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/sys v0.47.0
)

require github.com/jfreymuth/pulse v0.1.3 // indirect

replace github.com/sinshu/go-meltysynth => github.com/Distortions81/go-meltysynth v0.1.2

require (
	github.com/Distortions81/GoBeep86 v0.0.1
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.0 // indirect
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
)
