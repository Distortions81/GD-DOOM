# Sound System Worker Plan

## Goal

Replace the current mostly immediate, fire-and-forget sound playback path with a worker-owned request and channel system that can support:

- more realistic spatial mixing
- delayed playback for sourceport-style sound travel time
- live pan/volume updates while a sound is still playing
- channel limits, priorities, and voice stealing
- cleaner transition handling and less gameplay/audio coupling

## Why Change It

The current model is simple, but it has hard limits:

- gameplay code still owns too much of the scheduling
- delayed playback is game-side and tic-oriented
- active sounds are not first-class runtime objects
- spatialization is mostly decided at start time
- richer mixing features would require more one-off paths

This is good enough for basic Doom playback, but it is the wrong shape for more realistic or more controllable audio mixing.

## Current Shape

Today the system looks like this:

1. gameplay queues `soundEvent`s
2. gameplay optionally attaches world origins
3. gameplay flushes the queue during update
4. `soundSystem` immediately creates `audio.Player`s
5. playback becomes mostly fire-and-forget

That means there is not really a separate sound worker yet. There is only a queue on the game side and immediate playback on the sound side.

## Target Shape

### 1. Play Requests

Gameplay should submit a small request object, not directly control playback timing.

Suggested fields:

- `event`
- `sampleID` or resolved sample reference
- `positioned`
- `originX`
- `originY`
- `fullVolume`
- `priority`
- `delayMS`
- `sourceportOnly`

The main thread should only describe what to play and any important metadata.

### 2. Sound Worker Ownership

The sound system should own:

- pending play requests
- active channels/voices
- due-time scheduling
- active player lifecycle
- per-frame parameter updates

That gives one place to reason about timing, voice replacement, and spatial behavior.

### 3. Channels / Voices

Use a fixed or bounded channel pool instead of unlimited fresh players.

Each active channel should track:

- current sample/player
- start time
- whether it is positioned
- origin
- current left/right gains
- priority
- whether it is interruptible

This is the minimum structure needed for realistic mixing behavior.

### 4. Live Spatial Updates

For positioned sounds, the worker should be able to recompute:

- volume
- stereo separation

while the sound is still active.

That is a better fit for:

- moving listener
- moving actor
- long sounds like doors or monster deaths

It is also a better base for sourceport-only sound travel delay.

## Rollout Plan

### Phase 1: Internal Refactor With No Behavior Change

- introduce a `playRequest` type
- route current event emission through request submission
- move pending scheduling into `soundSystem`
- keep `delayMS = 0`
- keep current audible behavior the same

This phase should be low-risk and mostly architectural.

### Phase 2: Channel Ownership

- add active channel records
- stop treating every sound as an isolated one-shot
- move stop/cleanup logic fully into the worker
- keep current volume/pan math unless it needs cleanup

### Phase 3: Live Parameter Updates

- update positioned sounds while playing
- allow listener movement to affect active sounds
- keep Doom-faithful defaults where needed

This is the point where the system starts becoming noticeably more robust.

### Phase 4: Optional Sourceport Features

Add features that should not affect strict Doom parity by default:

- sound travel delay
- stronger distance models
- more advanced channel limiting
- future occlusion/reverb experiments if desired

These should be optional and default-safe.

## Non-Goals

This refactor should not try to do everything at once.

Not part of the initial pass:

- full environmental reverb
- HRTF/binaural processing
- physically correct acoustics
- format conversion redesign
- music driver rewrite

## Parity Notes

For strict Doom behavior:

- immediate playback remains the default
- no sound travel delay by default
- Doom-style attenuation and stereo rules remain the baseline

The worker/channel design is about architecture first. It should not force sourceport behavior into the default path.

## Risks

- transition bugs if old and new queueing paths overlap
- duplicate sounds if requests are submitted twice during migration
- active channel cleanup bugs
- timing drift if due-time scheduling is mixed with tic-based assumptions

The first implementation should favor correctness and inspectability over cleverness.

## Validation

When this work starts, validate in stages:

1. no audible regressions in current Doom-like playback
2. level transitions do not replay or leak old sounds
3. positioned world sounds still pan and attenuate correctly
4. monster death/alert/attack sounds still trigger at the right moments
5. optional sourceport travel delay only affects sourceport mode

## Follow-Up

Once the worker/channel model exists, re-evaluate these remaining sound items:

- monster movement/state sounds like `hoof` and `metal`
- per-event timing/routing parity details
- richer startup sound import diagnostics
