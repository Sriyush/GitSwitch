import { esc } from '../../lib/dom.js';
import { icon } from '../../lib/icons.js';
import { avatar } from '../../lib/avatar.js';
import { tilde } from '../../lib/paths.js';
import { state } from '../../lib/store.js';

// The active profile is rendered large at the top of the screen, the rest as a
// list below it. Same component either way: the difference between "who I am"
// and "who I could be" is emphasis, not different information.
export function profileCard(p, { hero = false } = {}) {
  const key = state.keys[p.name];
  return `
    <article class="card ${hero ? 'hero' : ''}">
      <div class="card-head">
        <div class="who-block">
          ${avatar(p.name, hero ? 'lg' : '')}
          <div class="names">
            <h2>${esc(p.name)}${hero ? '<span class="badge">active</span>' : ''}</h2>
            <p>${esc(p.git_name)} &lt;${esc(p.git_email)}&gt;</p>
          </div>
        </div>
        <div class="actions">
          ${hero ? '' : `<button data-act="switch" data-name="${esc(p.name)}">${icon('swap')} Switch</button>`}
          <button class="ghost" data-act="key" data-name="${esc(p.name)}">${icon('key')} ${key ? 'Hide key' : 'Key'}</button>
          <button class="ghost" data-act="edit" data-name="${esc(p.name)}">${icon('edit')} Edit</button>
          <button class="ghost danger icon" data-act="delete" data-name="${esc(p.name)}"
            aria-label="Remove ${esc(p.name)}" title="Remove ${esc(p.name)}">${icon('trash')}</button>
        </div>
      </div>

      <dl class="facts">
        ${fact('GitHub', esc(p.username))}
        ${fact('Directory', p.root
          ? `<span class="path" title="${esc(p.root)}">${esc(tilde(p.root))}</span>`
          : '<em>not scoped</em>')}
        ${fact('SSH key', p.ssh_key
          ? `<span class="path" title="${esc(p.ssh_key)}">${esc(tilde(p.ssh_key))}</span>`
          : '<em>none</em>')}
        ${fact('Owners', p.orgs && p.orgs.length
          ? `<span class="chips">${p.orgs.map((o) => `<span class="chip">${esc(o)}</span>`).join('')}</span>`
          : '<em>none</em>')}
      </dl>

      <details class="more">
        <summary>More</summary>
        <dl class="facts">
          ${fact('Host', esc(p.host_alias))}
          ${fact('Signing', p.signing_key
            ? `<span class="path" title="${esc(p.signing_key)}">${esc(tilde(p.signing_key))}</span> <span class="dim">(${esc(p.signing_format)})</span>`
            : '<em>off</em>')}
          ${fact('Token', '<em>not supported yet — needs internal/keyring</em>')}
        </dl>
      </details>

      ${key ? keyBlock(p, key) : ''}
    </article>`;
}

function fact(label, value) {
  return `<div class="fact"><dt>${label}</dt><dd>${value}</dd></div>`;
}

// keyBlock exists to make the one thing this screen is for — getting the public
// key into GitHub — a copy and a click, rather than a careful drag-select of
// wrapped monospace text.
function keyBlock(p, key) {
  return `
    <div class="keybox">
      <div class="keybox-head">
        <span>Public key</span>
        <span class="actions">
          <button class="ghost" data-act="copy" data-name="${esc(p.name)}">${icon('copy')} Copy</button>
          <a class="btn ghost" href="https://github.com/settings/ssh/new" target="_blank" rel="noopener noreferrer">
            ${icon('external')} Add on GitHub
          </a>
        </span>
      </div>
      <pre class="key">${esc(key)}</pre>
      <p class="hint">Paste it at github.com/settings/ssh/new while signed in as <strong>${esc(p.username)}</strong>.</p>
    </div>`;
}
