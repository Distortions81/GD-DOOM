const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

// Exercise the unmodified browser script with a small DOM model; no browser,
// WASM build, network listener, or third-party test dependency is required.
class Element {
  constructor(tag = "div") {
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.listeners = new Map();
    this.parentElement = null;
    this.hidden = false;
    this.checked = false;
    this._value = "";
  }
  get value() { return this._value; }
  set value(value) {
    this._value = this.tagName === "SELECT" && !this.options.some((option) => option.value === value) ? "" : value;
  }
  get options() { return this.children; }
  get previousElementSibling() {
    return this.parentElement?.children[this.parentElement.children.indexOf(this) - 1] || null;
  }
  get nextElementSibling() {
    return this.parentElement?.children[this.parentElement.children.indexOf(this) + 1] || null;
  }
  append(...nodes) {
    for (const node of nodes) {
      if (node.parentElement) node.parentElement.children.splice(node.parentElement.children.indexOf(node), 1);
      this.children.push(node);
      node.parentElement = this;
    }
    if (this.tagName === "SELECT" && !this._value) this._value = this.children[0]?.value || "";
  }
  add(node) { this.append(node); }
  replaceChildren(...nodes) {
    for (const node of this.children) node.parentElement = null;
    this.children = [];
    this._value = "";
    this.append(...nodes);
  }
  insertBefore(node, before) {
    if (node.parentElement) node.parentElement.children.splice(node.parentElement.children.indexOf(node), 1);
    this.children.splice(this.children.indexOf(before), 0, node);
    node.parentElement = this;
  }
  querySelectorAll(selector) {
    return this.children.flatMap((child) => [
      ...(child.tagName === "INPUT" && (selector !== "input:checked" || child.checked) ? [child] : []),
      ...child.querySelectorAll(selector),
    ]);
  }
  setAttribute() {}
  focus() {}
  closest() {
    if (["BUTTON", "INPUT", "SELECT", "A", "TEXTAREA", "LABEL", "FORM"].includes(this.tagName) || this.id === "local-wad-panel") return this;
    return this.parentElement?.closest() || null;
  }
  addEventListener(name, listener) {
    if (!this.listeners.has(name)) this.listeners.set(name, []);
    this.listeners.get(name).push(listener);
  }
  dispatch(name, event = {}) {
    event.target ||= this;
    event.preventDefault ||= () => {};
    return Promise.all((this.listeners.get(name) || []).map((listener) => listener(event)));
  }
}

class Option extends Element {
  constructor(label, value) {
    super("option");
    this.textContent = label;
    this.value = value;
  }
}

function harness(search = "", userAgent = "desktop") {
  const nodes = new Map();
  for (const id of ["splash", "game-shell", "start-button", "fullscreen-button", "local-wad-button", "local-wad-input", "local-wad-panel", "local-wad-base", "local-wad-overlays", "local-wad-apply", "build-pill", "status-announcer"]) {
    const tag = id === "local-wad-base" ? "select" : id.endsWith("button") ? "button" : "div";
    nodes.set(id, Object.assign(new Element(tag), { id }));
  }
  nodes.get("local-wad-base").add(new Option("Shareware", "DOOM1.WAD"));
  const shell = nodes.get("game-shell");
  shell.contentWindow = { focus() {}, postMessage() {} };
  const window = new Element();
  window.location = { href: `https://play.example/index.html${search}`, search, origin: "https://play.example", replace(url) { this.replaced = url; } };
  const document = new Element();
  document.getElementById = (id) => nodes.get(id);
  document.createElement = (tag) => new Element(tag);
  document.createTextNode = (text) => Object.assign(new Element("#text"), { textContent: text });
  const context = vm.createContext({ window, document, navigator: { userAgent }, Element, Option, URL, URLSearchParams, Uint8Array, TextEncoder });
  vm.runInContext(fs.readFileSync(path.join(__dirname, "launch.js"), "utf8"), context);
  return {
    nodes, window, shell,
    load(files) { context.files = files; return vm.runInContext("loadLocalWADFiles(files)", context); },
    run(source) { return vm.runInContext(source, context); },
    inputs() { return nodes.get("local-wad-overlays").querySelectorAll("input"); },
    url() { return new URL(shell.src); },
  };
}

function wad(name, kind = "PWAD", marker = 0) {
  const bytes = new Uint8Array(12);
  bytes.set(new TextEncoder().encode(kind));
  bytes[4] = marker;
  return { name, size: bytes.length, arrayBuffer: async () => bytes.buffer };
}

test("oversized batches are rejected before reading file bytes", async () => {
  for (const sizes of [Array(17).fill(12), Array(3).fill(64 * 1024 * 1024)]) {
    const h = harness();
    let reads = 0;
    const files = sizes.map((size, i) => ({ name: `file${i}.wad`, size, arrayBuffer: async () => { reads++; return new ArrayBuffer(12); } }));
    await assert.rejects(h.load(files), /at most 16 WADs totaling 128 MiB/);
    assert.equal(reads, 0);
    assert.equal(h.window.__gddoomLocalWADs.length, 0);
  }
});

test("adding WADs preserves overlay order and excluded files", async () => {
  const h = harness();
  await h.load([wad("base.wad", "IWAD"), wad("a.wad"), wad("b.wad"), wad("c.wad")]);
  const list = h.nodes.get("local-wad-overlays");
  h.inputs()[1].checked = false;
  list.insertBefore(list.children[2], list.children[0]);
  await h.load([wad("d.wad")]);
  assert.deepEqual(h.inputs().map((input) => [input.value, input.checked]), [
    ["browser-upload/c.wad", true], ["browser-upload/a.wad", true],
    ["browser-upload/b.wad", false], ["browser-upload/d.wad", true],
  ]);
  assert.equal(h.nodes.get("local-wad-base").value, "browser-upload/base.wad");
});

test("replacement file kinds remove stale base and overlay options", async () => {
  const h = harness();
  await h.load([wad("base.wad", "IWAD"), wad("map.wad")]);
  await h.load([wad("base.wad", "PWAD")]);
  assert.equal(h.nodes.get("local-wad-base").options.length, 1);
  assert.equal(h.nodes.get("local-wad-base").value, "DOOM1.WAD");
  assert.equal(h.inputs().length, 2);
});

test("a superseded slow read cannot overwrite a newer selection or input", async () => {
  const h = harness();
  let release;
  const old = wad("same.wad", "PWAD", 1);
  const oldBytes = await old.arrayBuffer();
  old.arrayBuffer = () => new Promise((resolve) => { release = () => resolve(oldBytes); });
  const pending = h.load([old]);
  await h.load([wad("same.wad", "IWAD", 2)]);
  h.nodes.get("local-wad-input").value = "next-selection";
  release();
  assert.equal(await pending, false);
  assert.equal(h.window.__gddoomLocalWADs[0].bytes[4], 2);
  assert.equal(h.window.__gddoomLocalWADs[0].kind, "IWAD");
  assert.equal(h.nodes.get("local-wad-input").value, "next-selection");
});

test("launch actions wait for loading instead of applying stale selection", async () => {
  const h = harness();
  let release;
  const file = wad("later.wad");
  const bytes = await file.arrayBuffer();
  file.arrayBuffer = () => new Promise((resolve) => { release = () => resolve(bytes); });
  const pending = h.load([file]);
  const before = h.shell.src;
  h.run("claimFocusAndStart()");
  assert.equal(h.nodes.get("splash").hidden, false);
  assert.equal(h.shell.src, before);
  assert.match(h.nodes.get("status-announcer").textContent, /Wait/);
  release();
  await pending;
});

for (const action of ["start", "splash", "enter"]) {
  test(`${action} uses current base and ordered overlays without requiring Apply`, async () => {
    const h = harness();
    await h.load([wad("base.wad", "IWAD"), wad("first.wad"), wad("second.wad")]);
    const rows = h.nodes.get("local-wad-overlays");
    rows.insertBefore(rows.children[1], rows.children[0]);
    switch (action) {
      case "start": await h.nodes.get("start-button").dispatch("click"); break;
      case "splash": await h.nodes.get("splash").dispatch("click"); break;
      case "enter": await h.window.dispatch("keydown", { key: "Enter" }); break;
    }
    assert.equal(h.url().searchParams.get("wad"), "browser-upload/base.wad");
    assert.equal(h.url().searchParams.get("file"), "browser-upload/second.wad,browser-upload/first.wad");
    assert.equal(h.nodes.get("splash").hidden, true);
  });
}

const oldJoinQuery = "?connect=wss%3A%2F%2Fhost.example%2Fnetplay&player-name=Player&spectate=true&multiplayer-server=wss%3A%2F%2Fhost.example%2Fnetplay&multiplayer-lobby=https%3A%2F%2Flobby.example&map=E1M3&skill=4&no-monsters=true&wad=DOOM1.WAD&file=custom.wad";

for (const userAgent of ["desktop", "iPhone"]) {
  test(`${userAgent} launcher only forwards content from old direct-join links`, async () => {
    const h = harness(oldJoinQuery, userAgent);
    const url = userAgent === "desktop" ? h.url() : new URL(h.window.location.replaced);
    assert.deepEqual([...url.searchParams], [["wad", "DOOM1.WAD"], ["file", "custom.wad"]]);
    await h.nodes.get("start-button").dispatch("click");
    assert.equal(h.url().searchParams.has("connect"), false);
  });
}

test("opening player.html directly cannot bypass setup through join or map parameters", async () => {
  const source = fs.readFileSync(path.join(__dirname, "player.html"), "utf8").match(/<script>([\s\S]*?)<\/script>/)[1];
  const window = new Element();
  window.location = { href: `https://play.example/player.html${oldJoinQuery}`, search: oldJoinQuery };
  window.parent = window;
  window.requestAnimationFrame = () => {};
  // Even stale build metadata must not inject multiplayer launch arguments.
  window.__gddoomMultiplayerServer = "wss://old.example/netplay";
  window.__gddoomMultiplayerLobby = "https://old.example";
  const document = {
    getElementById: () => ({ remove() {} }),
    createElement: () => ({}),
    head: { appendChild: (script) => script.onload() },
  };
  let argv;
  class Go {
    run() { argv = Array.from(this.argv); }
  }
  await vm.runInNewContext(source, {
    window, document, Go, URL, URLSearchParams, Uint8Array,
    fetch: async () => ({ ok: true, arrayBuffer: async () => new ArrayBuffer(8) }),
    WebAssembly: { instantiate: async () => ({ instance: {} }) },
    console: { error(error) { throw error; } },
  });
  assert.deepEqual(argv, ["gddoom", "-music-backend=impsynth", "-wad=DOOM1.WAD", "-file=custom.wad"]);
});

test("ready messages preserve load controls and selection before play intent", async () => {
  const h = harness();
  await h.load([wad("base.wad", "IWAD"), wad("map.wad")]);
  for (const type of ["gddoom-player-ready", "gddoom-session-started"]) {
    await h.window.dispatch("message", { origin: h.window.location.origin, source: h.shell.contentWindow, data: { type } });
  }
  assert.equal(h.nodes.get("local-wad-button").hidden, false);
  assert.match(h.nodes.get("status-announcer").textContent, /Choose the base game/);
  h.run("claimFocusAndStart()");
  assert.equal(h.nodes.get("local-wad-button").hidden, true);
});

test("editing a previously applied stack is captured on start", async () => {
  const h = harness();
  await h.load([wad("a.wad"), wad("b.wad")]);
  assert.equal(h.run("applyLocalWADSelection()"), true);
  h.inputs()[0].checked = false;
  await h.nodes.get("start-button").dispatch("click");
  assert.equal(h.url().searchParams.get("file"), "browser-upload/b.wad");
});

test("clicking overlay labels and panel text does not dismiss the selection form", async () => {
  const h = harness();
  await h.load([wad("map.wad")]);
  const before = h.shell.src;
  await h.nodes.get("splash").dispatch("click", { target: h.inputs()[0].parentElement });
  await h.nodes.get("splash").dispatch("click", { target: h.nodes.get("local-wad-panel") });
  assert.equal(h.nodes.get("splash").hidden, false);
  assert.equal(h.shell.src, before);
});

test("failed batch leaves the previously loaded store intact", async () => {
  const h = harness();
  await h.load([wad("existing.wad", "IWAD", 7)]);
  await assert.rejects(h.load([wad("existing.wad", "IWAD", 8), wad("broken.wad", "bad!")]), /not a WAD/);
  assert.equal(h.window.__gddoomLocalWADs.length, 1);
  assert.equal(h.window.__gddoomLocalWADs[0].bytes[4], 7);
});
