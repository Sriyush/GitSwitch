// Inline 16px stroke icons.
//
// Inline rather than a sprite or a font: they are embedded in a Go binary and
// must not cost a second request, and stroke="currentColor" is what lets one
// definition work on a green button, a red error row, and a dim label.
const PATHS = {
  user: '<circle cx="8" cy="5.5" r="2.8"/><path d="M2.5 14c0-2.8 2.5-4.4 5.5-4.4s5.5 1.6 5.5 4.4"/>',
  folder: '<path d="M1.8 4.2h4.3l1.3 1.6h6.8v7.4a.8.8 0 0 1-.8.8H2.6a.8.8 0 0 1-.8-.8z"/>',
  stethoscope: '<path d="M4.5 1.8v3.4a3 3 0 0 0 6 0V1.8"/><path d="M7.5 8.2v2.3a3.3 3.3 0 0 0 6.6 0V9"/><circle cx="14.1" cy="7.6" r="1.4"/>',
  key: '<circle cx="5.2" cy="8" r="3"/><path d="M8.2 8h6M12 8v2.4M14.2 8v1.8"/>',
  check: '<path d="M2.8 8.4l3.4 3.4 7-7.6"/>',
  cross: '<path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/>',
  alert: '<path d="M8 2.2 1.6 13.4h12.8z"/><path d="M8 6.4v3M8 11.4v.1"/>',
  minus: '<path d="M3.5 8h9"/>',
  plus: '<path d="M8 3v10M3 8h10"/>',
  copy: '<rect x="5.5" y="5.5" width="8" height="8" rx="1.4"/><path d="M10.5 3.5a1.4 1.4 0 0 0-1.4-1h-5a1.4 1.4 0 0 0-1.4 1.4v5c0 .6.4 1.1 1 1.3"/>',
  edit: '<path d="M11.2 2.6 13.4 4.8 5.6 12.6 2.6 13.4l.8-3z"/>',
  trash: '<path d="M2.8 4.4h10.4M6.2 4.4V2.8h3.6v1.6M4.4 4.4l.7 8.4h5.8l.7-8.4"/>',
  swap: '<path d="M2.6 5.4h9.2M9.4 2.8l2.6 2.6-2.6 2.6"/><path d="M13.4 10.6H4.2M6.6 13.2 4 10.6l2.6-2.6"/>',
  external: '<path d="M9.4 2.6h4v4M13.4 2.6 7.6 8.4"/><path d="M12 9.6v3.2a.9.9 0 0 1-.9.9H3.3a.9.9 0 0 1-.9-.9V5a.9.9 0 0 1 .9-.9h3.2"/>',
  x: '<path d="M4 4l8 8M12 4l-8 8"/>',
};

export function icon(name, cls = '') {
  return `<svg class="i ${cls}" viewBox="0 0 16 16" width="16" height="16" fill="none"
    stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"
    aria-hidden="true">${PATHS[name] || ''}</svg>`;
}
