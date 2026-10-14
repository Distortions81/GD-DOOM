// Deterministic tests for the exact AudioWorklet source embedded by driver_js.go.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "driver_worklet.js"), "utf8");

class Simulation {
  constructor(rate = 44100) {
    this.rate = rate;
    this.pending = null;
    this.requestCount = 0;
    this.produced = 0;
    this.played = 0;
    this.allocations = 0;
    this.delay = () => 6 * 128; // Ordinary main-thread response within a 60 Hz frame.
    const simulation = this;
    let Processor;
    class FakeAudioWorkletProcessor {
      constructor() {
        this.port = {
          postMessage(message) {
            assert.equal(simulation.pending, null, "more than one outstanding request");
            assert(message.frames > 0 && message.frames <= simulation.blockFrames);
            const number = ++simulation.requestCount;
            simulation.pending = {
              frames: message.frames,
              due: simulation.processor.clockFrame_ + simulation.delay(number),
            };
          },
        };
      }
    }
    class CountedFloat32Array extends Float32Array {
      constructor(...args) {
        super(...args);
        simulation.allocations++;
      }
    }
    vm.runInNewContext(source, {
      sampleRate: rate,
      Float32Array: CountedFloat32Array,
      AudioWorkletProcessor: FakeAudioWorkletProcessor,
      registerProcessor(name, constructor) {
        assert.equal(name, "oto-worklet-processor");
        Processor = constructor;
      },
    });
    this.blockFrames = Math.min(1024, Math.max(128, Math.floor(rate / 40 / 128) * 128));
    this.processor = new Processor({ processorOptions: { channelCount: 2, blockFrames: this.blockFrames } });
    this.ring = this.processor.buffer_;
  }

  tick() {
    const p = this.processor;
    // Zero-delay replies can fill the reservoir in a bounded sequence of
    // messages before the next audio quantum. No audio is skipped to catch up.
    let replies = 0;
    while (this.pending && this.pending.due <= p.clockFrame_) {
      assert(++replies <= Math.ceil(p.maximumFrames_ / this.blockFrames) + 1);
      const { frames } = this.pending;
      this.pending = null;
      const data = new Float32Array(frames * 2);
      for (let frame = 0; frame < frames; frame++) {
        data[frame * 2] = ++this.produced;
        data[frame * 2 + 1] = -this.produced;
      }
      p.port.onmessage({ data });
    }
    const left = new Float32Array(128);
    const right = new Float32Array(128);
    assert.equal(p.process([], [[left, right]]), true);
    for (let frame = 0; frame < left.length; frame++) {
      if (left[frame] === 0) {
        assert.equal(right[frame], 0);
        continue;
      }
      assert.equal(left[frame], ++this.played, "PCM frame repeated, discarded, or reordered");
      assert.equal(right[frame], -this.played, "stereo channels lost alignment");
    }
    assert.equal(this.produced, this.played + p.queuedFrames_, "queue lost audio");
    assert(p.queuedFrames_ >= 0 && p.queuedFrames_ <= p.maximumFrames_);
    assert(p.targetFrames_ >= p.minimumFrames_ && p.targetFrames_ <= p.maximumFrames_);
    assert.equal(p.waitRecv_, this.pending !== null);
    assert.equal(p.buffer_, this.ring, "ring reallocated during playback");
    assert.equal(this.allocations, 1, "worklet allocated another audio buffer");
  }

  run(seconds) {
    const quanta = Math.ceil(seconds * this.rate / 128);
    for (let i = 0; i < quanta; i++) this.tick();
  }
}

test("normal frame cadence keeps the small reservoir and uninterrupted FIFO audio", () => {
  const sim = new Simulation();
  sim.run(4);
  assert(sim.played > sim.rate * 3);
  assert.equal(sim.processor.targetFrames_, sim.processor.minimumFrames_);
  assert.equal(sim.processor.underruns_, 0);
  assert(sim.processor.minimumFrames_ / sim.rate >= 0.050);
  assert(sim.processor.minimumFrames_ / sim.rate <= 0.070);
});

test("occasional slow replies grow headroom and cover later stalls", () => {
  const sim = new Simulation();
  sim.run(1);
  sim.delay = (number) => (number % 20 === 0 ? 24 : 3) * 128;
  sim.run(3);
  assert(sim.processor.targetFrames_ > sim.processor.minimumFrames_);
  const underruns = sim.processor.underruns_;
  sim.run(6);
  assert.equal(sim.processor.underruns_, underruns, "learned headroom did not cover repeated scheduling stalls");
});

test("long stalls cap memory and resume remaining audio without priming again", () => {
  const sim = new Simulation();
  sim.run(1);
  let delayed = false;
  sim.delay = () => {
    if (delayed) return 0;
    delayed = true;
    return sim.rate; // One-second main-thread stall cannot be buffered away.
  };
  sim.run(2);
  assert(sim.processor.underruns_ > 0);
  assert.equal(sim.processor.targetFrames_, sim.processor.maximumFrames_);
  assert(sim.processor.maximumFrames_ / sim.rate <= 0.200);
  assert.equal(sim.processor.priming_, false);
  assert(sim.played > sim.rate);
});

test("recovery shrinks gradually after stable responses without dropping queued samples", () => {
  const sim = new Simulation();
  sim.run(1);
  let delayed = false;
  sim.delay = () => {
    if (delayed) return 3 * 128;
    delayed = true;
    return 24 * 128;
  };
  sim.run(2);
  const grown = sim.processor.targetFrames_;
  assert(grown > sim.processor.minimumFrames_);
  sim.run(7);
  assert.equal(sim.processor.targetFrames_, grown, "shrank before ten stable seconds");
  sim.run(2);
  assert(sim.processor.targetFrames_ < grown);
  assert(sim.processor.targetFrames_ > sim.processor.minimumFrames_);
  sim.run(20);
  assert.equal(sim.processor.targetFrames_, sim.processor.minimumFrames_);
});

test("consecutive underrun quanta count one shortage until PCM resumes", () => {
  const sim = new Simulation();
  sim.run(1);
  sim.delay = () => sim.rate;
  sim.run(0.4);
  assert.equal(sim.processor.underruns_, 1);
  const underruns = sim.processor.underruns_;
  sim.run(0.2);
  assert.equal(sim.processor.underruns_, underruns);
});

test("reservoir bounds and FIFO hold at other browser sample rates", () => {
  for (const rate of [11025, 22050, 48000, 96000]) {
    const sim = new Simulation(rate);
    // Keep transfer throughput faster than real time at every sample rate.
    sim.delay = () => 128;
    sim.run(2);
    assert(sim.played > rate);
    assert.equal(sim.processor.underruns_, 0);
    assert(sim.processor.maximumFrames_ / rate <= 0.200);
  }
});
