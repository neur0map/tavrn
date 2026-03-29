const lines = [
  "pull up a chair \u2022 warm lights \u2022 low music",
  "the tavern is open now \u2022 ssh tavrn.sh",
  "no signup \u2022 no browser \u2022 just ssh",
];

const node = document.getElementById("status-line");
let index = 0;

setInterval(() => {
  index = (index + 1) % lines.length;
  node.textContent = lines[index];
}, 2400);

// ── Tavern Radio ──
const audio = document.getElementById("audio");
const playBtn = document.getElementById("play-btn");
const nowPlaying = document.getElementById("now-playing");
const radioDiv = document.getElementById("radio");
let playing = false;
let pollTimer = null;

// Check if web audio is available
fetch("/now-playing")
  .then(r => r.json())
  .then(data => {
    if (data.playing) {
      radioDiv.classList.remove("hidden");
      updateNowPlaying(data);
    }
  })
  .catch(() => {});

function updateNowPlaying(data) {
  if (data.playing) {
    nowPlaying.textContent = "\u266A " + data.title + " \u2022 " + data.artist;
  } else {
    nowPlaying.textContent = "";
  }
}

function pollNowPlaying() {
  fetch("/now-playing")
    .then(r => r.json())
    .then(updateNowPlaying)
    .catch(() => {});
}

// Handle stream interruptions — reconnect automatically
audio.addEventListener("ended", () => {
  if (playing) {
    audio.src = "/stream?" + Date.now();
    audio.play().catch(() => {});
  }
});

audio.addEventListener("error", () => {
  if (playing) {
    setTimeout(() => {
      audio.src = "/stream?" + Date.now();
      audio.play().catch(() => {});
    }, 2000);
  }
});

if (playBtn) {
  playBtn.addEventListener("click", () => {
    if (playing) {
      audio.pause();
      audio.src = "";
      playing = false;
      playBtn.textContent = "\u25B6 Listen Live";
      if (pollTimer) clearInterval(pollTimer);
    } else {
      audio.src = "/stream";
      audio.play();
      playing = true;
      playBtn.textContent = "\u25A0 Stop";
      pollTimer = setInterval(pollNowPlaying, 5000);
    }
  });
}
