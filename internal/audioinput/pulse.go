package audioinput

import (
	"fmt"
	"strings"
)

const (
	defaultFrameDurationMS   = 30
	defaultPulseSampleRate   = 48000
	defaultPulseChannels     = 1
	defaultPulseFormat       = "s16le"
	defaultPulseLatencyMilli = 30
)

type PulseConfig struct {
	Device        string
	SampleRate    int
	Channels      int
	Format        string
	LatencyMillis int
}

func (c PulseConfig) normalized() PulseConfig {
	if c.SampleRate <= 0 {
		c.SampleRate = defaultPulseSampleRate
	}
	if c.Channels <= 0 {
		c.Channels = defaultPulseChannels
	}
	if strings.TrimSpace(c.Format) == "" {
		c.Format = defaultPulseFormat
	}
	if c.LatencyMillis <= 0 {
		c.LatencyMillis = defaultPulseLatencyMilli
	}
	c.Device = strings.TrimSpace(c.Device)
	c.Format = strings.ToLower(strings.TrimSpace(c.Format))
	return c
}

func (c PulseConfig) validate() error {
	if c.SampleRate <= 0 {
		return fmt.Errorf("pulse sample rate must be > 0")
	}
	if (c.SampleRate*defaultFrameDurationMS)%1000 != 0 {
		return fmt.Errorf("pulse sample rate %d must divide evenly into %d ms frames", c.SampleRate, defaultFrameDurationMS)
	}
	if c.Channels <= 0 {
		return fmt.Errorf("pulse channels must be > 0")
	}
	if c.LatencyMillis <= 0 {
		return fmt.Errorf("pulse latency must be > 0 ms")
	}
	switch c.Format {
	case "s16le":
		return nil
	default:
		return fmt.Errorf("unsupported pulse format %q", c.Format)
	}
}

func (c PulseConfig) samplesPerFrame() int {
	return c.SampleRate * defaultFrameDurationMS / 1000
}

func (c PulseConfig) bytesPerSample() int {
	switch c.Format {
	case "s16le":
		return 2
	default:
		return 0
	}
}

func (c PulseConfig) bytesPerFrame() int {
	return c.samplesPerFrame() * c.Channels * c.bytesPerSample()
}
