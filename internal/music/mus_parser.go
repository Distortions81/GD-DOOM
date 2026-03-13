package music

import (
	"encoding/binary"
	"fmt"
)

const (
	musSig = "MUS\x1a"
)

type musHeader struct {
	sig               [4]byte
	scoreLen          uint16
	scoreStart        uint16
	primaryChannels   uint16
	secondaryChannels uint16
	instrumentCount   uint16
	reserved          uint16
}

// ParseMUS decodes a Doom MUS stream into driver Events.
func ParseMUS(data []byte) ([]Event, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("mus: data too short: %d", len(data))
	}
	var h musHeader
	copy(h.sig[:], data[:4])
	if string(h.sig[:]) != musSig {
		return nil, fmt.Errorf("mus: bad signature %q", string(h.sig[:]))
	}
	h.scoreLen = binary.LittleEndian.Uint16(data[4:6])
	h.scoreStart = binary.LittleEndian.Uint16(data[6:8])
	h.primaryChannels = binary.LittleEndian.Uint16(data[8:10])
	h.secondaryChannels = binary.LittleEndian.Uint16(data[10:12])
	h.instrumentCount = binary.LittleEndian.Uint16(data[12:14])
	h.reserved = binary.LittleEndian.Uint16(data[14:16])

	pos := int(h.scoreStart)
	if pos < 0 || pos >= len(data) {
		return nil, fmt.Errorf("mus: invalid score start %d", pos)
	}
	scoreEnd := pos + int(h.scoreLen)
	if int(h.scoreLen) > 0 && scoreEnd <= len(data) {
		// Clamp reads to the score payload when length is valid.
		data = data[:scoreEnd]
	}

	vel := [16]uint8{}
	for i := range vel {
		vel[i] = 127
	}
	chMap := musChannelMap{}
	var out []Event
	var pendingDelta uint32

	appendEvent := func(ev Event) {
		ev.DeltaTics = pendingDelta
		pendingDelta = 0
		out = append(out, ev)
	}

	for pos < len(data) {
		evb := data[pos]
		pos++
		last := (evb & 0x80) != 0
		rawCh := evb & 0x0F
		musType := (evb >> 4) & 0x07
		ch := chMap.midiChannel(rawCh)

		ev := Event{Channel: ch}
		switch musType {
		case 0: // release note
			if pos >= len(data) {
				return nil, fmt.Errorf("mus: truncated note-off")
			}
			ev.Type = EventNoteOff
			ev.A = data[pos] & 0x7F
			pos++
			appendEvent(ev)
		case 1: // play note
			if pos >= len(data) {
				return nil, fmt.Errorf("mus: truncated note-on")
			}
			n := data[pos]
			pos++
			ev.Type = EventNoteOn
			ev.A = n & 0x7F
			if (n & 0x80) != 0 {
				if pos >= len(data) {
					return nil, fmt.Errorf("mus: truncated note-on velocity")
				}
				vel[ch] = data[pos] & 0x7F
				pos++
			}
			ev.B = vel[ch]
			appendEvent(ev)
		case 2: // pitch wheel
			if pos >= len(data) {
				return nil, fmt.Errorf("mus: truncated pitch bend")
			}
			// MUS pitch wheel is an 8-bit value. The legacy conversion path
			// expands this to the MIDI wheel range by multiplying by 64.
			b := data[pos]
			pos++
			p := uint16(b) << 6 // 0..8064 (MUS style)
			ev.Type = EventPitchBend
			ev.A = uint8(p & 0x7F)
			ev.B = uint8((p >> 7) & 0x7F)
			appendEvent(ev)
		case 3: // system event
			if pos >= len(data) {
				return nil, fmt.Errorf("mus: truncated system event")
			}
			sys := data[pos]
			pos++
			cc, ok := musSystemToControl(sys)
			if ok {
				ev.Type = EventControlChange
				ev.A = cc
				ev.B = 0
				appendEvent(ev)
			}
		case 4: // change controller
			if pos+1 >= len(data) {
				return nil, fmt.Errorf("mus: truncated controller")
			}
			ctrl := data[pos]
			val := data[pos+1] & 0x7F
			pos += 2
			if ctrl == 0 {
				ev.Type = EventProgramChange
				ev.A = val
				appendEvent(ev)
				break
			}
			cc, ok := musControllerToMIDI(ctrl)
			if ok {
				ev.Type = EventControlChange
				ev.A = cc
				ev.B = val
				appendEvent(ev)
			}
		case 5: // end of measure (unused)
			// no-op
		case 6: // end of score
			appendEvent(Event{Type: EventEnd, Channel: ch})
			return out, nil
		default:
			return nil, fmt.Errorf("mus: unsupported event type %d", musType)
		}

		if last {
			delta, n, err := readMUSVarLen(data[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
			pendingDelta = delta
		}
	}
	return out, nil
}

func readMUSVarLen(data []byte) (uint32, int, error) {
	var v uint32
	for i := 0; i < len(data); i++ {
		b := data[i]
		v = (v << 7) | uint32(b&0x7F)
		if (b & 0x80) == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("mus: truncated delta-time")
}

func musControllerToMIDI(c uint8) (uint8, bool) {
	switch c {
	case 1:
		return 32, true // bank select (DMX mus2mid uses CC 0x20)
	case 2:
		return 1, true // modulation
	case 3:
		return 7, true // volume
	case 4:
		return 10, true // pan
	case 5:
		return 11, true // expression
	case 6:
		return 91, true // reverb
	case 7:
		return 93, true // chorus
	case 8:
		return 64, true // sustain
	case 9:
		return 67, true // soft pedal
	default:
		return 0, false
	}
}

func musSystemToControl(c uint8) (uint8, bool) {
	switch c {
	case 10:
		return 120, true // all sounds off
	case 11:
		return 123, true // all notes off
	case 12:
		return 126, true // mono
	case 13:
		return 127, true // poly
	case 14:
		return 121, true // reset all controllers
	default:
		return 0, false
	}
}

type musChannelMap struct {
	next byte
	m    [16]byte
	set  [16]bool
}

func (cm *musChannelMap) midiChannel(musCh byte) byte {
	// MUS channel 15 is percussion and maps to MIDI 9.
	if musCh == 15 {
		return 9
	}
	if cm.set[musCh] {
		return cm.m[musCh]
	}
	c := cm.next
	if c == 9 {
		c++
	}
	cm.m[musCh] = c
	cm.set[musCh] = true
	cm.next = c + 1
	if cm.next == 9 {
		cm.next++
	}
	return c
}
