import { esc } from './dom.js';

// A monogram per profile.
//
// This tool exists because two accounts look alike until you have already
// pushed with the wrong one. Giving each a stable colour and initial makes them
// tell apart at a glance, which no amount of reading `you@acme.com` in 12px
// monospace ever does. The hue is derived from the name, so it is the same on
// every machine and after every restart, with no state to store.
const HUES = [8, 32, 45, 96, 150, 190, 220, 268, 320];

export function avatar(name, size = '') {
  const hue = HUES[hash(name) % HUES.length];
  return `<span class="avatar ${size}" style="--h:${hue}" aria-hidden="true">${esc(initials(name))}</span>`;
}

function initials(name) {
  const parts = String(name).split(/[-_. ]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return String(name).slice(0, 2).toUpperCase();
}

function hash(s) {
  let h = 0;
  for (const ch of String(s)) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return h;
}
