const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");

let splashDismissed = false;
let pendingReload = false;

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
    case "gddoom-webgl-context-lost":
      reloadPlayer();
      break;
    default:
      break;
  }
});
