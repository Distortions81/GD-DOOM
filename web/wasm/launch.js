const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");
const utilityBar = document.querySelector(".utilitybar");
const localWADButton = document.getElementById("local-wad-button");
const localWADInput = document.getElementById("local-wad-input");
const localWADStatus = document.getElementById("local-wad-status");

let splashDismissed = false;
let pendingReload = false;
let localWADStatusTimer = 0;

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
  if (!shell || !shell.contentWindow) {
    return;
  }
  shell.focus({ preventScroll: true });
  shell.contentWindow.postMessage({ type: "gddoom-focus-canvas" }, window.location.origin);
}

function setLocalWADStatus(text) {
  if (!localWADStatus) {
    return;
  }
  localWADStatus.textContent = text || "";
}

function flashLocalWADStatus(text, durationMs = 2600) {
  if (localWADStatusTimer) {
    window.clearTimeout(localWADStatusTimer);
    localWADStatusTimer = 0;
  }
  setLocalWADStatus(text);
  if (!text || durationMs <= 0) {
    return;
  }
  localWADStatusTimer = window.setTimeout(() => {
    localWADStatusTimer = 0;
    setLocalWADStatus("");
  }, durationMs);
}

function getLocalWADStore() {
  if (!Array.isArray(window.__gddoomLocalWADs)) {
    window.__gddoomLocalWADs = [];
  }
  return window.__gddoomLocalWADs;
}

function updateLocalWADStatus() {
  flashLocalWADStatus("", 0);
}

function hideLocalWADUI() {
  if (utilityBar) {
    utilityBar.hidden = true;
  }
}

async function loadLocalWADFiles(fileList) {
  const files = Array.from(fileList || []).filter((file) => /\.wad$/i.test(file.name));
  if (!files.length) {
    setLocalWADStatus("No .wad files selected.");
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
  flashLocalWADStatus(`Loaded ${noun}. Reloading picker...`);
  reloadPlayer();
}

function reloadPlayer() {
  if (!shell || pendingReload) {
    return;
  }
  pendingReload = true;
  const url = new URL(shell.src, window.location.href);
  url.searchParams.set("reload", String(Date.now()));
  shell.src = url.toString();
}

function claimFocusAndStart() {
  focusPlayer();
  hideSplash();
}

if (splash) {
  splash.addEventListener("click", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  });
}

if (localWADButton && localWADInput) {
  localWADButton.addEventListener("click", () => {
    localWADInput.click();
  });
  localWADInput.addEventListener("change", async () => {
    try {
      await loadLocalWADFiles(localWADInput.files);
    } catch (err) {
      flashLocalWADStatus(`Load failed: ${err instanceof Error ? err.message : String(err)}`, 4200);
    } finally {
      localWADInput.value = "";
    }
  });
  updateLocalWADStatus();
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
      break;
    case "gddoom-session-started":
      hideLocalWADUI();
      flashLocalWADStatus("", 0);
      break;
    case "gddoom-webgl-context-lost":
      reloadPlayer();
      break;
    default:
      break;
  }
});
