import { esc } from './dom.js';
import { state } from './store.js';

// Paths are stored and sent absolute — git and ssh read them literally, with no
// shell to expand a tilde — but a card full of absolute paths is unreadable.
// The home prefix is folded for display only; the full value stays in the title
// attribute and in every form field.
export function tilde(path) {
  if (!path || !state.home) return path || '';
  if (path === state.home) return '~';
  return path.startsWith(state.home + '/') ? '~' + path.slice(state.home.length) : path;
}

// foldHome is tilde() for prose: doctor details and remedies are sentences with
// paths embedded in them, not path fields, so the prefix is folded wherever it
// appears rather than only at the start.
export function foldHome(text) {
  if (!text || !state.home) return text || '';
  return String(text).split(state.home + '/').join('~/');
}

export function pathCell(path, placeholder) {
  if (!path) return `<em>${esc(placeholder)}</em>`;
  return `<span class="path" title="${esc(path)}">${esc(tilde(path))}</span>`;
}
