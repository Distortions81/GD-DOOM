const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");
const startButton = document.getElementById("start-button");
const fullscreenButton = document.getElementById("fullscreen-button");
const localWADButton = document.getElementById("local-wad-button");
const localWADInput = document.getElementById("local-wad-input");
const buildPill = document.getElementById("build-pill");
const statusAnnouncer = document.getElementById("status-announcer");
const multiplayerForm = document.getElementById("multiplayer-form");
const multiplayerServer = document.getElementById("multiplayer-server");
const multiplayerName = document.getElementById("multiplayer-name");
const multiplayerWAD = document.getElementById("multiplayer-wad");
const multiplayerRole = document.getElementById("multiplayer-role");
const joinStatus = document.getElementById("join-status");
let multiplayerLaunch = null;

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
  for (const name of ["connect", "multiplayer-server", "player-name", "spectate", "map", "skill", "no-monsters", "wad", "file"]) {
    if (launchParams.has(name)) url.searchParams.set(name, launchParams.get(name));
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
  return Boolean(target.closest("a, button, input, select, textarea, summary, [role='button'], [tabindex]"));
}

function hideSplash() {
  if (splashDismissed || !splash) {
    return;
  }
  splashDismissed = true;
  splash.hidden = true;
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
  const files = Array.from(fileList || []).filter((file) => /\.wad$/i.test(file.name));
  if (!files.length) {
    return;
  }

  const store = getLocalWADStore();
  const loadedNames = [];
  for (const file of files) {
    const bytes = new Uint8Array(await file.arrayBuffer());
    const path = `browser-upload/${file.name}`;
    const nextEntry = { path, name: file.name, bytes };
    loadedNames.push(file.name);
    const existingIndex = store.findIndex((entry) => String(entry.path || "").toLowerCase() === path.toLowerCase());
    if (existingIndex >= 0) {
      store.splice(existingIndex, 1, nextEntry);
    } else {
      store.push(nextEntry);
    }
  }

  const noun = loadedNames.length === 1 ? "IWAD file" : "IWAD files";
  if (multiplayerWAD) {
    for (const entry of store) {
      if (!Array.from(multiplayerWAD.options).some((option) => option.value === entry.path)) {
        multiplayerWAD.add(new Option(entry.name, entry.path));
      }
    }
  }
  setStatus(`Loaded ${noun}. Reloading picker.`);
  reloadPlayer();
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
    } finally {
      localWADInput.value = "";
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
  if (event.origin !== window.location.origin || !event.data) {
    return;
  }
  switch (event.data.type) {
    case "gddoom-player-ready":
      pendingReload = false;
      if (hasLocalWADs()) {
        hideLocalWADControls();
        setStatus("");
      }
      if (splashDismissed) {
        focusPlayer();
      }
      break;
    case "gddoom-session-started":
      hideLocalWADControls();
      setStatus("");
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
