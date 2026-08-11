// gitswitch web UI — the shell that owns the tabs, the event routing, and the
// render loop.
//
// Native ES modules, no framework and no bundler: this ships embedded in a Go
// binary and must build with no toolchain in front of it. See web/embed.go.
// Every feature is a module under features/ exporting the same shape —
// { id, label, render(), actions, forms, onEnter? } — so adding a screen is
// adding a folder and one line in FEATURES, not surgery on this file.
import { TOKEN, STALE } from './lib/api.js';
import { subscribe } from './lib/events.js';
import { $, banner, esc } from './lib/dom.js';
import { icon } from './lib/icons.js';
import { avatar } from './lib/avatar.js';
import { state, refresh, onChange, notify } from './lib/store.js';
import profiles from './features/profiles/index.js';
import paths from './features/paths/index.js';
import doctor from './features/doctor/index.js';

const FEATURES = [profiles, paths, doctor];
const byId = Object.fromEntries(FEATURES.map((f) => [f.id, f]));
const current = () => byId[state.view];

// The nav is generated from the feature list, so a new screen never means
// editing index.html as well.
$('#tabs').innerHTML = FEATURES.map((f) => `
  <button role="tab" data-view="${f.id}" aria-selected="${f.id === state.view}"
    class="${f.id === state.view ? 'on' : ''}">${icon(f.icon)} ${esc(f.label)}</button>`).join('');

// --- render ----------------------------------------------------------------

function render() {
  const active = state.profiles.find((p) => p.active);
  $('#active-line').innerHTML = active
    ? `${avatar(active.name)}<div class="who"><b>${esc(active.name)}</b><span>${esc(active.git_email)}</span></div>`
    : `<div class="who"><b>${state.loaded ? 'No active profile' : 'Connecting…'}</b></div>`;

  document.querySelectorAll('#tabs button').forEach((b) => {
    const on = b.dataset.view === state.view;
    b.classList.toggle('on', on);
    b.setAttribute('aria-selected', on ? 'true' : 'false');
  });

  // A blank page while the first fetch is in flight reads as a broken server.
  $('#view').innerHTML = state.loaded
    ? current().render()
    : '<article class="card skeleton"><span></span><span></span><span></span></article>';
}

onChange(render);

// --- routing and event delegation ------------------------------------------
//
// One listener per event type on the document, dispatching by data-act into the
// current feature's action map. Delegation is what lets render() replace the
// whole view with innerHTML without ever rebinding a handler.

document.addEventListener('click', (e) => {
  const tab = e.target.closest('#tabs button');
  if (tab) {
    show(tab.dataset.view);
    return;
  }

  const btn = e.target.closest('button[data-act]');
  if (!btn) return;

  if (btn.dataset.act === 'dismiss') { banner(''); return; }

  const action = current().actions[btn.dataset.act];
  if (action) action({ name: btn.dataset.name, btn });
});

document.addEventListener('submit', (e) => {
  e.preventDefault();
  const form = e.target;
  const handler = current().forms[form.id] || current().forms[[...form.classList].find((c) => current().forms[c])];
  if (!handler) return;

  const data = Object.fromEntries(new FormData(form).entries());
  handler({
    form,
    data,
    orgs: (data.orgs || '').split(',').map((s) => s.trim()).filter(Boolean),
    submit: form.querySelector('button[type=submit]'),
  });
});

// Escape backs out of an edit, which is what every other form on a desktop does.
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && state.editing) { state.editing = null; notify(); }
});

function show(view) {
  state.view = view;
  banner(''); // an error from another screen is not about this one
  render();
  current().onEnter?.();
}

// --- start -----------------------------------------------------------------

// A page loaded without the token cannot do anything, and every request it
// makes will 401. Say that plainly rather than letting the user read an
// authentication error as a bug.
if (!TOKEN) {
  state.loaded = true;
  render();
  banner(
    'No session token. The UI only works from the address `gsw ui` prints, since the ' +
    'token is minted per run and never written to disk. Open that link — a reload of ' +
    'it keeps working, but a bookmark of this bare address will not.',
    'error',
  );
} else {
  render(); // paint the shell immediately; the first fetch fills it in
  subscribe(
    () => { refresh().catch(() => {}); },
    () => banner(STALE, 'error'),
  );
  refresh().catch((err) => banner(err.message, 'error'));
}
