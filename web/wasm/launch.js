const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");
const startButton = document.getElementById("start-button");
const fullscreenButton = document.getElementById("fullscreen-button");
const localWADButton = document.getElementById("local-wad-button");
const localWADInput = document.getElementById("local-wad-input");
const localWADPanel = document.getElementById("local-wad-panel");
const localWADBase = document.getElementById("local-wad-base");
const localWADOverlays = document.getElementById("local-wad-overlays");
const localWADApply = document.getElementById("local-wad-apply");
const buildPill = document.getElementById("build-pill");
const statusAnnouncer = document.getElementById("status-announcer");
const multiplayerForm = document.getElementById("multiplayer-form");
const multiplayerServer = document.getElementById("multiplayer-server");
const multiplayerName = document.getElementById("multiplayer-name");
const multiplayerWAD = document.getElementById("multiplayer-wad");
const multiplayerRole = document.getElementById("multiplayer-role");
const joinStatus = document.getElementById("join-status");
let multiplayerLaunch = null;
let localWADLaunch = null;
let localWADLoadGeneration = 0;
let localWADLoading = false;
let localWADStoreVersion = 0;

let splashDismissed = false;
let pendingReload = false;

function setStatus(text) {
  if (statusAnnouncer) {
    statusAnnouncer.textContent = text || "";
  }
}

function getBuildID() {
  return typeof window.__gddoomBuildID === "string" ? window.__gddoomBuildID : "";
}

function isMobileLike() {
  if (typeof navigator === "undefined") {
    return false;
  }
  return /Android|iPad|iPhone|iPod/i.test(navigator.userAgent) || (
    navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1
  );
}

function getPlayerURL() {
  const url = new URL("./player.html", window.location.href);
  const launchParams = new URLSearchParams(window.location.search);
  for (const name of ["connect", "multiplayer-server", "multiplayer-lobby", "player-name", "spectate", "map", "skill", "no-monsters", "wad", "file"]) {
    if (launchParams.has(name)) url.searchParams.set(name, launchParams.get(name));
  }
  if (localWADLaunch) {
    url.searchParams.set("wad", localWADLaunch.wad);
    if (localWADLaunch.files.length) url.searchParams.set("file", localWADLaunch.files.join(","));
    else url.searchParams.delete("file");
  }
  if (multiplayerLaunch) {
    url.searchParams.set("connect", multiplayerLaunch.address);
    url.searchParams.set("player-name", multiplayerLaunch.name);
    url.searchParams.set("wad", multiplayerLaunch.wad);
    url.searchParams.set("spectate", String(multiplayerLaunch.spectator));
  }
  const buildID = getBuildID();
  if (buildID) {
    url.searchParams.set("v", buildID);
  }
  return url.toString();
}

function startDirectPlayer() {
  window.location.replace(getPlayerURL());
}

function isInteractiveTarget(target) {
  if (!(target instanceof Element)) {
    return false;
  }
  return Boolean(target.closest("a, button, input, select, textarea, summary, label, form, #local-wad-panel, [role='button'], [tabindex]"));
}

function hideSplash() {
  if (splashDismissed || !splash) {
    return;
  }
  splashDismissed = true;
  splash.hidden = true;
  hideLocalWADControls();
}

function focusPlayer() {
  if (!shell) {
    return;
  }
  try {
    shell.focus({ preventScroll: true });
  } catch (_err) {
  }
  if (!shell.contentWindow) {
    return;
  }
  try {
    shell.contentWindow.focus();
    shell.contentWindow.postMessage({ type: "gddoom-claim-focus" }, window.location.origin);
  } catch (_err) {
  }
}

async function requestFullscreen() {
  if (!shell) {
    return;
  }
  const target = shell;
  const request = target.requestFullscreen || target.webkitRequestFullscreen;
  if (typeof request !== "function") {
    return;
  }
  try {
    await request.call(target);
    focusPlayer();
  } catch (_err) {
  }
}

function getLocalWADStore() {
  if (!Array.isArray(window.__gddoomLocalWADs)) {
    window.__gddoomLocalWADs = [];
  }
  return window.__gddoomLocalWADs;
}

function hasLocalWADs() {
  return getLocalWADStore().length > 0;
}

function hideLocalWADControls() {
  if (localWADButton) {
    localWADButton.hidden = true;
  }
  if (localWADInput) {
    localWADInput.hidden = true;
  }
}

async function loadLocalWADFiles(fileList) {
  const generation = ++localWADLoadGeneration;
  localWADLoading = true;
  try {
    const files = Array.from(fileList || []).filter((file) => /\.wad$/i.test(file.name));
    if (!files.length) return false;
    const store = getLocalWADStore();
    const selectedFiles = new Map();
    const plannedSizes = new Map(store.map((entry) => [entry.path.toLowerCase(), entry.bytes.length]));
    for (const file of files) {
      if (file.size > 64 * 1024 * 1024 || file.size < 12) throw new Error("Each WAD must be between 12 bytes and 64 MiB.");
      if (/[,/\\\u0000-\u001f\u007f-\u009f]/.test(file.name) || file.name.trim() !== file.name || Array.from(file.name).length > 64) {
        throw new Error("Use WAD filenames of at most 64 characters without commas, controls, or path separators.");
      }
      const key = `browser-upload/${file.name}`.toLowerCase();
      selectedFiles.set(key, file);
      plannedSizes.set(key, file.size);
    }
    // Reject oversized batches before allocating any file buffers. Replacing
    // an existing name counts only its replacement size.
    if (plannedSizes.size > 16 || Array.from(plannedSizes.values()).reduce((sum, size) => sum + size, 0) > 128 * 1024 * 1024) {
      throw new Error("Load at most 16 WADs totaling 128 MiB.");
    }
    const staged = [];
    for (const file of selectedFiles.values()) {
      const bytes = new Uint8Array(await file.arrayBuffer());
      if (generation !== localWADLoadGeneration) return false;
      if (bytes.length !== file.size) throw new Error(`${file.name} changed while it was being read.`);
      const kind = String.fromCharCode(...bytes.subarray(0, 4));
      if (kind !== "IWAD" && kind !== "PWAD") throw new Error(`${file.name} is not a WAD file.`);
      staged.push({ path: `browser-upload/${file.name}`, name: file.name, bytes, kind });
    }
    const nextStore = store.slice();
    for (const nextEntry of staged) {
      const existingIndex = nextStore.findIndex((entry) => entry.path.toLowerCase() === nextEntry.path.toLowerCase());
      if (existingIndex >= 0) nextStore.splice(existingIndex, 1, nextEntry);
      else nextStore.push(nextEntry);
    }
    store.splice(0, store.length, ...nextStore);
    localWADStoreVersion++;
    renderLocalWADSelection(staged);
    localWADPanel.hidden = false;
    setStatus("WAD files loaded. Choose the base game and custom map order, then use selected WADs.");
    return true;
  } catch (err) {
    if (generation !== localWADLoadGeneration) return false;
    throw err;
  } finally {
    if (generation === localWADLoadGeneration) {
      localWADLoading = false;
      localWADInput.value = "";
    }
  }
}

function renderLocalWADSelection(staged) {
  const store = getLocalWADStore();
  const selectedBase = localWADBase.value;
  const previous = Array.from(localWADOverlays.querySelectorAll("input"), (input) => ({ key: input.value.toLowerCase(), checked: input.checked }));
  const overlays = new Map(store.filter((entry) => entry.kind === "PWAD").map((entry) => [entry.path.toLowerCase(), entry]));
  const ordered = [];
  for (const selection of previous) {
    if (overlays.has(selection.key)) {
      ordered.push({ entry: overlays.get(selection.key), checked: selection.checked });
      overlays.delete(selection.key);
    }
  }
  for (const entry of overlays.values()) ordered.push({ entry, checked: true });
  localWADBase.replaceChildren(new Option("Doom shareware", "DOOM1.WAD"));
  if (multiplayerWAD) multiplayerWAD.replaceChildren(new Option("Doom shareware", "DOOM1.WAD"));
  localWADOverlays.replaceChildren();
  for (const entry of store) {
    if (entry.kind === "IWAD") {
      localWADBase.add(new Option(entry.name, entry.path));
      if (multiplayerWAD) multiplayerWAD.add(new Option(entry.name, entry.path));
    }
  }
  for (const { entry, checked } of ordered) {
    const row = document.createElement("li");
    const label = document.createElement("label");
    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.checked = checked;
    checkbox.value = entry.path;
    label.append(checkbox, document.createTextNode(` ${entry.name}`));
    row.append(label);
    for (const [direction, text] of [[-1, "Up"], [1, "Down"]]) {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = text;
      button.setAttribute("aria-label", `Move ${entry.name} ${text.toLowerCase()}`);
      button.addEventListener("click", () => {
        if (direction < 0 && row.previousElementSibling) localWADOverlays.insertBefore(row, row.previousElementSibling);
        if (direction > 0 && row.nextElementSibling) localWADOverlays.insertBefore(row.nextElementSibling, row);
      });
      row.append(button);
    }
    localWADOverlays.append(row);
  }
  const newBase = staged.find((entry) => entry.kind === "IWAD");
  localWADBase.value = newBase ? newBase.path : selectedBase;
  if (!localWADBase.value) localWADBase.value = "DOOM1.WAD";
  if (multiplayerWAD) multiplayerWAD.value = localWADBase.value;
}

function captureLocalWADSelection(base = localWADBase.value) {
  const selection = {
    wad: base,
    files: Array.from(localWADOverlays.querySelectorAll("input:checked"), (input) => input.value),
    version: localWADStoreVersion,
  };
  if (selection.files.length + 1 > 16) throw new Error("Select at most 15 custom WADs in addition to the base game.");
  const changed = !localWADLaunch || JSON.stringify(localWADLaunch) !== JSON.stringify(selection);
  localWADLaunch = selection;
  return changed;
}

function applyLocalWADSelection(forceReload = true) {
  if (localWADLoading) {
    setStatus("Wait for the selected WAD files to finish loading.");
    return false;
  }
  if (!hasLocalWADs() || !localWADBase) return true;
  let changed;
  try {
    changed = captureLocalWADSelection();
  } catch (err) {
    setStatus(err.message);
    return false;
  }
  if (multiplayerWAD) multiplayerWAD.value = localWADLaunch.wad;
  if (changed || forceReload) {
    multiplayerLaunch = null;
    pendingReload = false;
    reloadPlayer();
    setStatus("Loading selected WADs. Open Multiplayer to create or join a game.");
  }
  return true;
}

function reloadPlayer() {
  if (!shell || pendingReload) {
    return;
  }
  pendingReload = true;
  const url = new URL(getPlayerURL());
  url.searchParams.set("reload", String(Date.now()));
  shell.src = url.toString();
}

function initializePlayerFrame() {
  if (!shell) {
    return;
  }
  const target = getPlayerURL();
  if (shell.src !== target) {
    shell.src = target;
  }
}

function claimFocusAndStart() {
  if (!applyLocalWADSelection(false)) return;
  hideSplash();
  focusPlayer();
}

function updateBuildPill() {
  if (!buildPill) {
    return;
  }
  const buildID = getBuildID();
  buildPill.textContent = buildID ? `Build: ${buildID}` : "Build: local";
}

updateBuildPill();
if (multiplayerServer) {
  const defaults = new URLSearchParams(window.location.search);
  multiplayerServer.value = defaults.get("connect") || defaults.get("multiplayer-server") || window.__gddoomMultiplayerServer || "";
}
initializePlayerFrame();

if (multiplayerForm) {
  multiplayerForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const address = multiplayerServer.value.trim();
    const name = multiplayerName.value.trim();
    if (!/^(https|wss|ws|wt):\/\//i.test(address)) {
      joinStatus.textContent = "Enter the server's HTTPS or WebSocket URL.";
      return;
    }
    if (!name || new TextEncoder().encode(name).length > 64) {
      joinStatus.textContent = "Choose a shorter player name.";
      return;
    }
    if (localWADLoading) {
      joinStatus.textContent = "Wait for the selected WAD files to finish loading.";
      return;
    }
    if (hasLocalWADs()) {
      try {
        captureLocalWADSelection(multiplayerWAD.value);
      } catch (err) {
        joinStatus.textContent = err.message;
        return;
      }
    }
    multiplayerLaunch = {address, name, wad: multiplayerWAD.value, spectator: multiplayerRole.value === "spectator"};
    joinStatus.textContent = "Connecting…";
    // A deliberate join replaces the idle single-player frame.
    pendingReload = false;
    reloadPlayer();
    hideSplash();
  });
}

if (isMobileLike()) {
  hideLocalWADControls();
  startDirectPlayer();
}

if (splash) {
  splash.addEventListener("click", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  });
  splash.addEventListener("touchstart", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  }, { passive: false });
}

if (startButton) {
  startButton.addEventListener("click", () => {
    claimFocusAndStart();
  });
}

if (localWADApply) localWADApply.addEventListener("click", () => applyLocalWADSelection());
if (localWADBase && multiplayerWAD) {
  localWADBase.addEventListener("change", () => { multiplayerWAD.value = localWADBase.value; });
  multiplayerWAD.addEventListener("change", () => { localWADBase.value = multiplayerWAD.value; });
}

if (fullscreenButton) {
  fullscreenButton.addEventListener("click", () => {
    requestFullscreen();
  });
}

if (localWADButton && localWADInput && !isMobileLike()) {
  localWADButton.addEventListener("click", () => {
    localWADInput.click();
  });
  localWADInput.addEventListener("change", async () => {
    try {
      await loadLocalWADFiles(localWADInput.files);
    } catch (err) {
      setStatus(`Load failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  });
}

window.addEventListener("keydown", (event) => {
  if (isInteractiveTarget(event.target)) {
    return;
  }
  if (event.key !== "Enter" && event.key !== " " && event.key !== "Spacebar") {
    return;
  }
  if (splashDismissed) {
    return;
  }
  event.preventDefault();
  claimFocusAndStart();
});

window.addEventListener("message", (event) => {
  if (event.origin !== window.location.origin || !shell || event.source !== shell.contentWindow || !event.data) {
    return;
  }
  switch (event.data.type) {
    case "gddoom-player-ready":
      pendingReload = false;
      if (splashDismissed) {
        hideLocalWADControls();
        setStatus("");
      }
      if (splashDismissed) {
        focusPlayer();
      }
      break;
    case "gddoom-session-started":
      if (splashDismissed) {
        hideLocalWADControls();
        setStatus("");
      }
      break;
    case "gddoom-webgl-context-lost":
      reloadPlayer();
      break;
    default:
      break;
  }
});

document.addEventListener("fullscreenchange", () => {
  if (!document.fullscreenElement && splashDismissed) {
    focusPlayer();
  }
});
