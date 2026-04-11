package sound

import (
	"encoding/binary"
	"fmt"
	"io"
)

var pcSpeakerCaptureMagic = [8]byte{'G', 'D', 'P', 'C', 'S', 'P', 'K', '1'}

type PCSpeakerCapture struct {
	TickRate int
	Tones    []PCSpeakerTone
}

func WritePCSpeakerCapture(w io.Writer, capture PCSpeakerCapture) error {
	if w == nil {
		return fmt.Errorf("nil writer")
	}
	if capture.TickRate <= 0 {
		return fmt.Errorf("invalid tick rate %d", capture.TickRate)
	}
	if _, err := w.Write(pcSpeakerCaptureMagic[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(capture.TickRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(capture.Tones))); err != nil {
		return err
	}
	var rec [4]byte
	for _, tone := range capture.Tones {
		rec[0] = 0
		if tone.Active {
			rec[0] = 1
		}
		rec[1] = tone.ToneValue
		binary.LittleEndian.PutUint16(rec[2:4], tone.Divisor)
		if _, err := w.Write(rec[:]); err != nil {
			return err
		}
	}
	return nil
}

func ReadPCSpeakerCapture(r io.Reader) (PCSpeakerCapture, error) {
	if r == nil {
		return PCSpeakerCapture{}, fmt.Errorf("nil reader")
	}
	var magic [8]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return PCSpeakerCapture{}, err
	}
	if magic != pcSpeakerCaptureMagic {
		return PCSpeakerCapture{}, fmt.Errorf("invalid pc speaker capture header")
	}
	var tickRate uint32
	if err := binary.Read(r, binary.LittleEndian, &tickRate); err != nil {
		return PCSpeakerCapture{}, err
	}
	if tickRate == 0 {
		return PCSpeakerCapture{}, fmt.Errorf("invalid tick rate 0")
	}
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return PCSpeakerCapture{}, err
	}
	tones := make([]PCSpeakerTone, count)
	var rec [4]byte
	for i := range tones {
		if _, err := io.ReadFull(r, rec[:]); err != nil {
			return PCSpeakerCapture{}, err
		}
		tones[i] = PCSpeakerTone{
			Active:    rec[0]&1 != 0,
			ToneValue: rec[1],
			Divisor:   binary.LittleEndian.Uint16(rec[2:4]),
		}
	}
	return PCSpeakerCapture{
		TickRate: int(tickRate),
		Tones:    tones,
	}, nil
}
