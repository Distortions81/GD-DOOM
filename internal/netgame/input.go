package netgame

import (
	"errors"
	"fmt"
	"math"

	"gddoom/internal/demo"
)

// Input is one player's intent for exactly one authoritative simulation tic.
// Sequence and Tick never wrap within a session epoch. A session must renew its
// epoch before either counter is exhausted. The session authenticates the
// player and epoch before handing an Input to its buffer.
type Input struct {
	Sequence uint32   `json:"sequence"`
	Tick     uint32   `json:"tick"`
	Command  demo.Tic `json:"command"`
}

const (
	// MaxInputWindow bounds memory and validation work even for hostile clients.
	MaxInputWindow = 256
	MaxForwardMove = 50
	// The existing controls permit strafe plus strafe-modified turn together.
	MaxSideMove = 80
)

var (
	ErrDuplicateInput      = errors.New("duplicate input")
	ErrConflictingInput    = errors.New("conflicting input")
	ErrStaleInput          = errors.New("input tic already finalized")
	ErrFutureInput         = errors.New("input outside future tic window")
	ErrInputSequence       = errors.New("input sequence is not increasing with tic")
	ErrInvalidInput        = errors.New("invalid input command")
	ErrInputTickOrder      = errors.New("input consumption must follow consecutive server tics")
	ErrInputEpochExhausted = errors.New("input epoch counters exhausted")
)

// InputBufferConfig is fixed for the lifetime of an input buffer. FutureTicks
// is the maximum offset from the next unfinalized tic, inclusive. Zero permits
// only the next tic. HoldTicks is how many missing tics may repeat movement.
type InputBufferConfig struct {
	FirstTick   uint32
	FutureTicks uint32
	HoldTicks   uint32
}

// InputAck separates finalized deadlines from commands actually received.
// Clients discard history through Tick even when its input was lost, and replay
// only inputs for later tics. Sequence must never acknowledge a future command
// merely because the network delivered it early. HasTick/HasSequence distinguish
// the initial state from valid counter zero.
type InputAck struct {
	Tick        uint32 `json:"tick"`
	Sequence    uint32 `json:"sequence"`
	HasTick     bool   `json:"has_tick"`
	HasSequence bool   `json:"has_sequence"`
}

// FinalizedInput contains the sole command to apply for a player's server tic.
// Sequence is meaningful only when Received is true. Synthesized movement is
// never mistaken for a received command acknowledgment.
type FinalizedInput struct {
	Tick     uint32
	Sequence uint32
	Command  demo.Tic
	Received bool
}

// InputBuffer is owned by the match simulation goroutine, not socket readers.
// It never waits for a client, reschedules late input, or drains a backlog into
// extra movement. Submit and Consume must not be called concurrently.
type InputBuffer struct {
	config       InputBufferConfig
	nextTick     uint32
	pending      map[uint32]Input
	ack          InputAck
	lastCommand  demo.Tic
	missingTicks uint32
	exhausted    bool
}

func NewInputBuffer(config InputBufferConfig) (*InputBuffer, error) {
	if config.FutureTicks > MaxInputWindow || config.HoldTicks > MaxInputWindow {
		return nil, fmt.Errorf("input window and hold duration must not exceed %d tics", MaxInputWindow)
	}
	return &InputBuffer{
		config:   config,
		nextTick: config.FirstTick,
		pending:  make(map[uint32]Input),
	}, nil
}

// ValidateInputCommand restricts untrusted intent to movement achievable by
// normal controls and gameplay buttons. Demo special commands (including pause
// and save) are not legal multiplayer intent. Weapon bits require Change; all
// eight demo weapon codes are valid (the last code directly selects chainsaw).
func ValidateInputCommand(command demo.Tic) error {
	if command.Forward < -MaxForwardMove || command.Forward > MaxForwardMove ||
		command.Side < -MaxSideMove || command.Side > MaxSideMove {
		return fmt.Errorf("%w: movement out of range", ErrInvalidInput)
	}
	const allowedButtons = demo.ButtonAttack | demo.ButtonUse | demo.ButtonChange | demo.ButtonWeaponMask
	if command.Buttons & ^byte(allowedButtons) != 0 {
		return fmt.Errorf("%w: unknown or special buttons", ErrInvalidInput)
	}
	weapon := command.Buttons & demo.ButtonWeaponMask
	if weapon != 0 && command.Buttons&demo.ButtonChange == 0 {
		return fmt.Errorf("%w: weapon bits without change button", ErrInvalidInput)
	}
	return nil
}

// Submit accepts out-of-order datagrams within a bounded future window.
// Retransmission of an identical pending input returns ErrDuplicateInput;
// callers may ignore that error. A consumed tic is immutable, even if its
// original command arrives later. Rejected commands never change the buffer.
func (b *InputBuffer) Submit(input Input) error {
	if b.exhausted || (b.ack.HasSequence && b.ack.Sequence == math.MaxUint32) {
		return ErrInputEpochExhausted
	}
	if input.Tick < b.nextTick {
		return ErrStaleInput
	}
	if uint64(input.Tick)-uint64(b.nextTick) > uint64(b.config.FutureTicks) {
		return ErrFutureInput
	}
	if err := ValidateInputCommand(input.Command); err != nil {
		return err
	}
	if previous, ok := b.pending[input.Tick]; ok {
		if previous == input {
			return ErrDuplicateInput
		}
		return ErrConflictingInput
	}
	if b.ack.HasSequence && input.Sequence <= b.ack.Sequence {
		return ErrInputSequence
	}
	for tick, pending := range b.pending {
		if pending.Sequence == input.Sequence {
			return ErrConflictingInput
		}
		if (tick < input.Tick && pending.Sequence > input.Sequence) ||
			(tick > input.Tick && pending.Sequence < input.Sequence) {
			return ErrInputSequence
		}
	}
	b.pending[input.Tick] = input
	return nil
}

// Consume finalizes exactly one tic without waiting. Missing inputs hold only
// forward/side movement briefly; turn, attack, use, and weapon selection clear
// immediately. The world must call this once for each consecutive server tic.
func (b *InputBuffer) Consume(tick uint32) (FinalizedInput, error) {
	if b.exhausted {
		return FinalizedInput{}, ErrInputEpochExhausted
	}
	if tick != b.nextTick {
		return FinalizedInput{}, ErrInputTickOrder
	}
	result := FinalizedInput{Tick: tick}
	if input, ok := b.pending[tick]; ok {
		delete(b.pending, tick)
		result.Sequence = input.Sequence
		result.Command = input.Command
		result.Received = true
		b.ack.Sequence = input.Sequence
		b.ack.HasSequence = true
		b.lastCommand = input.Command
		b.missingTicks = 0
	} else {
		// Saturation prevents a very long interruption from wrapping back into
		// the movement-hold interval.
		if b.missingTicks < math.MaxUint32 {
			b.missingTicks++
		}
		if b.missingTicks <= b.config.HoldTicks {
			result.Command.Forward = b.lastCommand.Forward
			result.Command.Side = b.lastCommand.Side
		}
	}
	b.ack.Tick = tick
	b.ack.HasTick = true
	if tick == math.MaxUint32 {
		b.exhausted = true
	} else {
		b.nextTick++
	}
	return result, nil
}

func (b *InputBuffer) Ack() InputAck { return b.ack }

func (b *InputBuffer) Pending() int { return len(b.pending) }
