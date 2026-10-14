package netgame

import (
	"errors"
	"math"
	"testing"

	"gddoom/internal/demo"
)

func newTestInputBuffer(t *testing.T, config InputBufferConfig) *InputBuffer {
	t.Helper()
	buffer, err := NewInputBuffer(config)
	if err != nil {
		t.Fatal(err)
	}
	return buffer
}

func submitInput(t *testing.T, buffer *InputBuffer, input Input) {
	t.Helper()
	if err := buffer.Submit(input); err != nil {
		t.Fatal(err)
	}
}

func consumeInput(t *testing.T, buffer *InputBuffer, tick uint32) FinalizedInput {
	t.Helper()
	input, err := buffer.Consume(tick)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestInputLossNeverHoldsActionsOrStopsWorld(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: 10, FutureTicks: 8, HoldTicks: 2})
	command := demo.Tic{Forward: 50, Side: -80, AngleTurn: 1200,
		Buttons: demo.ButtonAttack | demo.ButtonUse | demo.ButtonChange | 3<<demo.ButtonWeaponShift}
	submitInput(t, buffer, Input{Sequence: 4, Tick: 10, Command: command})
	first := consumeInput(t, buffer, 10)
	if first.Command != command || !first.Received || first.Sequence != 4 {
		t.Fatalf("first input: %+v", first)
	}
	for tick := uint32(11); tick <= 15; tick++ {
		got := consumeInput(t, buffer, tick)
		want := demo.Tic{}
		if tick <= 12 {
			want = demo.Tic{Forward: 50, Side: -80}
		}
		if got.Command != want || got.Received {
			t.Fatalf("tic %d: %+v, want command %+v and no receipt", tick, got, want)
		}
		if got := buffer.Ack(); got != (InputAck{Tick: tick, Sequence: 4, HasTick: true, HasSequence: true}) {
			t.Fatalf("tic %d acknowledgment: %+v", tick, got)
		}
	}
	if err := buffer.Submit(Input{Sequence: 5, Tick: 11, Command: command}); !errors.Is(err, ErrStaleInput) {
		t.Fatalf("late attack must not execute: %v", err)
	}
	submitInput(t, buffer, Input{Sequence: 10, Tick: 16, Command: demo.Tic{Forward: -25}})
	if got := consumeInput(t, buffer, 16); got.Command.Forward != -25 || !got.Received {
		t.Fatalf("resume from current tic: %+v", got)
	}
}

func TestInputFutureDeliveryDoesNotAcknowledgeOrAccelerate(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: 100, FutureTicks: 5})
	// Deliver a future burst backwards, as reordered redundant datagrams can.
	for tick := uint32(105); tick >= 100; tick-- {
		submitInput(t, buffer, Input{Sequence: tick + 1000, Tick: tick, Command: demo.Tic{Forward: 25}})
	}
	if got := buffer.Ack(); got.HasSequence || got.HasTick {
		t.Fatalf("delivery acknowledged future simulation: %+v", got)
	}
	for tick := uint32(100); tick <= 105; tick++ {
		input := consumeInput(t, buffer, tick)
		if input.Sequence != tick+1000 || input.Command.Forward != 25 {
			t.Fatalf("tic %d input: %+v", tick, input)
		}
		if got := buffer.Pending(); got != int(105-tick) {
			t.Fatalf("tic %d pending = %d", tick, got)
		}
		if got := buffer.Ack(); got.Tick != tick || got.Sequence != tick+1000 {
			t.Fatalf("tic %d acknowledgment: %+v", tick, got)
		}
		if _, err := buffer.Consume(tick); !errors.Is(err, ErrInputTickOrder) {
			t.Fatalf("same tic cannot execute twice: %v", err)
		}
	}
	if got := consumeInput(t, buffer, 106); got.Command != (demo.Tic{}) || got.Received {
		t.Fatalf("burst granted extra movement: %+v", got)
	}
}

func TestInputExpiredDeadlineRemovesLostCommandWithoutFalseSequenceAck(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: 0, FutureTicks: 4})
	submitInput(t, buffer, Input{Sequence: 900, Tick: 4})
	for tick := uint32(0); tick < 4; tick++ {
		consumeInput(t, buffer, tick)
		if got := buffer.Ack(); !got.HasTick || got.Tick != tick || got.HasSequence {
			t.Fatalf("lost tic %d acknowledgment: %+v", tick, got)
		}
	}
	consumeInput(t, buffer, 4)
	if got := buffer.Ack(); got != (InputAck{Tick: 4, Sequence: 900, HasTick: true, HasSequence: true}) {
		t.Fatalf("future input acknowledgment: %+v", got)
	}
}

func TestInputRejectsReplayConflictsAndSequenceReordering(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: 20, FutureTicks: 8})
	input := Input{Sequence: 100, Tick: 20, Command: demo.Tic{Forward: 25}}
	submitInput(t, buffer, input)
	tests := []struct {
		name  string
		input Input
		want  error
	}{
		{"duplicate", input, ErrDuplicateInput},
		{"replace command", Input{Sequence: 100, Tick: 20, Command: demo.Tic{Forward: 50}}, ErrConflictingInput},
		{"replace sequence", Input{Sequence: 101, Tick: 20, Command: input.Command}, ErrConflictingInput},
		{"replay at new tic", Input{Sequence: 100, Tick: 21}, ErrConflictingInput},
		{"older sequence at later tic", Input{Sequence: 99, Tick: 21}, ErrInputSequence},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := buffer.Submit(tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	if buffer.Pending() != 1 {
		t.Fatalf("invalid inputs changed buffer: %d", buffer.Pending())
	}
	if got := consumeInput(t, buffer, 20); got.Command != input.Command {
		t.Fatalf("conflict changed accepted command: %+v", got)
	}
	if err := buffer.Submit(Input{Sequence: 100, Tick: 21}); !errors.Is(err, ErrInputSequence) {
		t.Fatalf("finalized replay accepted: %v", err)
	}
	submitInput(t, buffer, Input{Sequence: 105, Tick: 25})
	if err := buffer.Submit(Input{Sequence: 106, Tick: 24}); !errors.Is(err, ErrInputSequence) {
		t.Fatalf("newer sequence at earlier tic: %v", err)
	}
	submitInput(t, buffer, Input{Sequence: 104, Tick: 24})
}

func TestInputFutureSpamIsBounded(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: 10, FutureTicks: 4})
	for tick := uint32(15); tick < 5000; tick++ {
		if err := buffer.Submit(Input{Sequence: tick, Tick: tick}); !errors.Is(err, ErrFutureInput) {
			t.Fatalf("tic %d spam error = %v", tick, err)
		}
	}
	for tick := uint32(10); tick <= 14; tick++ {
		submitInput(t, buffer, Input{Sequence: tick, Tick: tick})
	}
	if buffer.Pending() != 5 {
		t.Fatalf("pending = %d, want 5", buffer.Pending())
	}
	consumeInput(t, buffer, 10)
	submitInput(t, buffer, Input{Sequence: 15, Tick: 15})
	if buffer.Pending() != 5 {
		t.Fatalf("sliding window exceeded bound: %d", buffer.Pending())
	}
}

func TestInputCommandValidation(t *testing.T) {
	for _, command := range []demo.Tic{
		{}, {Forward: 50, Side: 80, AngleTurn: math.MaxInt16},
		{Forward: -50, Side: -80, AngleTurn: math.MinInt16},
		{Buttons: demo.ButtonAttack | demo.ButtonUse},
	} {
		if err := ValidateInputCommand(command); err != nil {
			t.Fatalf("valid command %+v: %v", command, err)
		}
	}
	for weapon := byte(0); weapon < 8; weapon++ {
		if err := ValidateInputCommand(demo.Tic{Buttons: demo.ButtonChange | weapon<<demo.ButtonWeaponShift}); err != nil {
			t.Fatalf("valid demo weapon code %d: %v", weapon, err)
		}
	}
	for _, command := range []demo.Tic{
		{Forward: 51}, {Forward: -51}, {Side: 81}, {Side: -81},
		{Buttons: demo.ButtonSpecial}, {Buttons: 64}, {Buttons: 8},
		{Buttons: demo.ButtonSpecial | demo.ButtonAttack},
	} {
		buffer := newTestInputBuffer(t, InputBufferConfig{})
		if err := buffer.Submit(Input{Command: command}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid command %+v: %v", command, err)
		}
		if got := consumeInput(t, buffer, 0); got.Received || got.Command != (demo.Tic{}) {
			t.Fatalf("invalid command affected simulation: %+v", got)
		}
	}
}

func TestInputCounterExhaustionRequiresNewEpoch(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{FirstTick: math.MaxUint32 - 1, FutureTicks: 4})
	if err := buffer.Submit(Input{Tick: 0}); !errors.Is(err, ErrStaleInput) {
		t.Fatalf("wrapped tic accepted: %v", err)
	}
	submitInput(t, buffer, Input{Sequence: math.MaxUint32 - 1, Tick: math.MaxUint32 - 1})
	submitInput(t, buffer, Input{Sequence: math.MaxUint32, Tick: math.MaxUint32})
	consumeInput(t, buffer, math.MaxUint32-1)
	consumeInput(t, buffer, math.MaxUint32)
	if _, err := buffer.Consume(0); !errors.Is(err, ErrInputEpochExhausted) {
		t.Fatalf("wrapped consume: %v", err)
	}
	if err := buffer.Submit(Input{Sequence: 0, Tick: 0}); !errors.Is(err, ErrInputEpochExhausted) {
		t.Fatalf("wrapped submit: %v", err)
	}
	buffer = newTestInputBuffer(t, InputBufferConfig{FutureTicks: 2})
	submitInput(t, buffer, Input{Sequence: math.MaxUint32, Tick: 0})
	consumeInput(t, buffer, 0)
	if err := buffer.Submit(Input{Sequence: 0, Tick: 1}); !errors.Is(err, ErrInputEpochExhausted) {
		t.Fatalf("wrapped sequence: %v", err)
	}
	// An exhausted client command sequence does not stall world tics.
	if got := consumeInput(t, buffer, 1); got.Received {
		t.Fatalf("exhausted command sequence delivered input: %+v", got)
	}
}

func TestInputZeroCountersAndTickOrdering(t *testing.T) {
	buffer := newTestInputBuffer(t, InputBufferConfig{})
	if _, err := buffer.Consume(1); !errors.Is(err, ErrInputTickOrder) {
		t.Fatalf("skipped simulation tic: %v", err)
	}
	submitInput(t, buffer, Input{})
	consumeInput(t, buffer, 0)
	if got := buffer.Ack(); got != (InputAck{HasTick: true, HasSequence: true}) {
		t.Fatalf("zero is a valid sequence and tic: %+v", got)
	}
}

func TestInputConfigurationBounds(t *testing.T) {
	for _, config := range []InputBufferConfig{
		{FutureTicks: MaxInputWindow + 1},
		{HoldTicks: MaxInputWindow + 1},
	} {
		if _, err := NewInputBuffer(config); err == nil {
			t.Fatalf("unbounded config accepted: %+v", config)
		}
	}
}
