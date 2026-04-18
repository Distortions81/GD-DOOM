package music

import "fmt"

// OPLSeqEvent stores one OPL register write and the delay until the next write.
// Delay is measured in MUS ticks at the driver's tic rate.
type OPLSeqEvent struct {
	Reg   uint16
	Value uint8
	Delay uint16
}

type oplSeqCaptureSynth struct {
	events       []OPLSeqEvent
	pendingDelay uint32
}

func (s *oplSeqCaptureSynth) Reset() {}

func (s *oplSeqCaptureSynth) WriteReg(addr uint16, value uint8) {
	delay := s.pendingDelay
	if delay > 0xFFFF {
		delay = 0xFFFF
	}
	s.events = append(s.events, OPLSeqEvent{
		Reg:   addr,
		Value: value,
		Delay: uint16(delay),
	})
	s.pendingDelay -= delay
}

func (s *oplSeqCaptureSynth) GenerateStereoS16(frames int) []int16 { return nil }

func (s *oplSeqCaptureSynth) GenerateMonoU8(frames int) []byte { return nil }

func (s *oplSeqCaptureSynth) AdvanceTicks(delta uint32) error {
	if delta == 0 {
		return nil
	}
	if len(s.events) == 0 {
		return fmt.Errorf("music: cannot encode leading delay without a preceding register write")
	}
	if uint64(s.pendingDelay)+uint64(delta) > 0xFFFF {
		return fmt.Errorf("music: OPL sequence delay overflow: %d", uint64(s.pendingDelay)+uint64(delta))
	}
	s.pendingDelay += delta
	return nil
}

func RenderParsedMUSToOPLSeq(parsed *ParsedMUS, bank PatchBank) ([]OPLSeqEvent, int, error) {
	if parsed == nil {
		return nil, defaultTicRate, nil
	}
	if bank == nil {
		bank = DefaultPatchBank{}
	}
	d, err := NewOutputDriverWithBackend(bank, BackendImpSynth)
	if err != nil {
		return nil, 0, err
	}
	capture := &oplSeqCaptureSynth{
		events: make([]OPLSeqEvent, 0, len(parsed.events)*8),
	}
	d.synth = capture
	d.Reset()
	for _, ev := range parsed.events {
		if err := capture.AdvanceTicks(ev.DeltaTics); err != nil {
			return nil, 0, err
		}
		d.applyEvent(ev)
	}
	out := make([]OPLSeqEvent, len(capture.events))
	copy(out, capture.events)
	return out, d.TicRate(), nil
}

func RenderMUSToOPLSeq(musData []byte, bank PatchBank) ([]OPLSeqEvent, int, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, 0, err
	}
	return RenderParsedMUSToOPLSeq(parsed, bank)
}
