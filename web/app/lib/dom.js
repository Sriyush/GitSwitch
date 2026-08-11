// Rendering primitives shared by every feature.

export const $ = (sel) => document.querySelector(sel);

// esc is applied to every value that reaches innerHTML. Profile fields are
// user-supplied strings, and one of them is a file path — quoting is not
// optional just because the data is local.
export function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}

let bannerTimer;

// banner is the single feedback channel: one message at a time, at the top of
// the page, where the eye already is after clicking something.
export function banner(msg, kind) {
  const el = $('#banner');
  clearTimeout(bannerTimer);
  el.innerHTML = msg
    ? `<span class="banner-msg">${esc(msg)}</span><button class="x" data-act="dismiss" aria-label="Dismiss">✕</button>`
    : '';
  el.className = 'banner ' + (kind || 'info');
  el.hidden = !msg;
  // Errors stay until dismissed or superseded: they carry a remedy worth
  // reading twice. Confirmations are noise once read.
  if (msg && kind !== 'error') bannerTimer = setTimeout(() => { el.hidden = true; }, 4000);
}

export async function guard(fn) {
  try { await fn(); } catch (err) { banner(err.message, 'error'); }
}

// busy disables the control that started a request. Without it a double-click
// posts twice, and two writers on profiles.json is exactly what the server
// serialises against — better not to send the second one at all.
export async function busy(el, fn) {
  if (el?.disabled) return; // already in flight
  // The work runs whether or not a control was found. Making it conditional on
  // the element is how a missing button turns into a form that silently does
  // nothing, which is a far worse failure than a button that stays enabled.
  if (el) el.disabled = true;
  try {
    await fn();
  } finally {
    if (el) el.disabled = false;
  }
}
