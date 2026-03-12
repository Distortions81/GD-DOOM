const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");

let splashDismissed = false;

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

function claimFocusAndStart() {
  focusPlayer();
  hideSplash();
}

if (splash) {
  splash.addEventListener("click", (event) => {
    event.preventDefault();
    claimFocusAndStart();
  });
}

window.addEventListener("keydown", (event) => {
  if (event.key !== "Enter" && event.key !== " " && event.key !== "Spacebar") {
    return;
  }
  if (splashDismissed) {
    return;
  }
  event.preventDefault();
  claimFocusAndStart();
});
