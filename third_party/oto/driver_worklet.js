// Copyright 2021 The Oto Authors
// Copyright 2026 GD-DOOM contributors
// SPDX-License-Identifier: Apache-2.0
// Modified from Oto v3.5.0's driver_js.go. See GDDOOM.md.

class OtoWorkletProcessor extends AudioWorkletProcessor {
  constructor(options) {
    super();
    const config = options.processorOptions;
    this.channelCount_ = config.channelCount;
    this.blockFrames_ = config.blockFrames;
    this.quantum_ = 128;
    this.minimumFrames_ = Math.ceil(sampleRate * 0.060 / this.quantum_) * this.quantum_;
    this.maximumFrames_ = Math.floor(sampleRate * 0.200 / this.quantum_) * this.quantum_;
    this.targetFrames_ = this.minimumFrames_;
    this.buffer_ = new Float32Array(this.maximumFrames_ * this.channelCount_);
    this.readFrame_ = 0;
    this.queuedFrames_ = 0;
    this.clockFrame_ = 0;
    this.requestFrame_ = 0;
    this.requestFrames_ = 0;
    this.waitRecv_ = false;
    this.priming_ = true;
    this.starving_ = false;
    this.stableFrame_ = -1;
    this.shrinkFrame_ = 0;
    this.underruns_ = 0;

    this.port.onmessage = (event) => {
      const data = event.data;
      const frames = data.length / this.channelCount_;
      // The main-thread producer replies with exactly the requested frame count.
      // Reject protocol errors rather than overwrite unread samples in the ring.
      if (!this.waitRecv_ || frames !== this.requestFrames_ ||
          frames + this.queuedFrames_ > this.maximumFrames_) {
        throw new Error("oto: invalid worklet audio reply");
      }
      const writeFrame = (this.readFrame_ + this.queuedFrames_) % this.maximumFrames_;
      const firstSamples = Math.min(frames, this.maximumFrames_ - writeFrame) * this.channelCount_;
      this.buffer_.set(data.subarray(0, firstSamples), writeFrame * this.channelCount_);
      if (firstSamples < data.length) {
        this.buffer_.set(data.subarray(firstSamples), 0);
      }
      this.queuedFrames_ += frames;
      this.waitRecv_ = false;
      this.observeDelay_(this.clockFrame_ - this.requestFrame_);
      this.requestIfNeeded_();
    };
  }

  observeDelay_(delayFrames) {
    const demand = this.blockFrames_ + 2 * delayFrames + this.quantum_;
    if (demand > this.targetFrames_) {
      this.grow_(demand);
      return;
    }
    if (this.targetFrames_ <= this.minimumFrames_ ||
        (demand > this.minimumFrames_ && demand > this.targetFrames_ * 2 / 3)) {
      this.stableFrame_ = -1;
      return;
    }
    if (this.stableFrame_ < 0) this.stableFrame_ = this.clockFrame_;
    if (this.clockFrame_ - this.stableFrame_ < 10 * sampleRate ||
        this.clockFrame_ - this.shrinkFrame_ < sampleRate) return;
    const reduction = Math.max(this.quantum_, Math.floor(this.targetFrames_ / 8 / this.quantum_) * this.quantum_);
    // Existing queued samples drain naturally; shrinking never discards audio.
    this.targetFrames_ = Math.max(this.minimumFrames_, this.targetFrames_ - reduction);
    this.shrinkFrame_ = this.clockFrame_;
  }

  grow_(frames) {
    this.targetFrames_ = Math.min(this.maximumFrames_, Math.ceil(frames / this.quantum_) * this.quantum_);
    this.stableFrame_ = -1;
    this.shrinkFrame_ = this.clockFrame_;
  }

  requestIfNeeded_() {
    if (this.waitRecv_) return;
    const missing = this.targetFrames_ - this.queuedFrames_;
    if (missing <= 0 || (!this.priming_ && missing < this.blockFrames_)) return;
    this.requestFrames_ = Math.min(this.blockFrames_, missing);
    this.requestFrame_ = this.clockFrame_;
    this.waitRecv_ = true;
    this.port.postMessage({ frames: this.requestFrames_ });
  }

  process(inputs, outputs) {
    const output = outputs[0];
    if (!output || output.length === 0) return true;
    const frames = output[0].length;
    // Initial priming absorbs the first main-thread scheduling gap. Afterwards
    // recovery continues immediately when samples arrive, without a new pause.
    if (this.priming_ && this.queuedFrames_ >= this.targetFrames_) this.priming_ = false;
    const available = this.priming_ ? 0 : Math.min(frames, this.queuedFrames_);
    for (let frame = 0; frame < available; frame++) {
      const offset = ((this.readFrame_ + frame) % this.maximumFrames_) * this.channelCount_;
      for (let channel = 0; channel < output.length; channel++) {
        output[channel][frame] = channel < this.channelCount_ ? this.buffer_[offset + channel] : 0;
      }
    }
    for (let channel = 0; channel < output.length; channel++) output[channel].fill(0, available);
    this.readFrame_ = (this.readFrame_ + available) % this.maximumFrames_;
    this.queuedFrames_ -= available;

    if (!this.priming_ && available < frames) {
      if (!this.starving_) {
        this.starving_ = true;
        this.underruns_++;
        const waiting = this.waitRecv_ ? this.clockFrame_ - this.requestFrame_ : 0;
        this.grow_(Math.max(this.targetFrames_ + this.blockFrames_, this.blockFrames_ + 2 * waiting + frames));
      }
    } else if (available === frames) {
      this.starving_ = false;
    }
    this.requestIfNeeded_();
    this.clockFrame_ += frames;
    return true;
  }
}

registerProcessor("oto-worklet-processor", OtoWorkletProcessor);
