// The path-mappings screen — the main reason the UI exists, since editing
// includeIf rules by hand is miserable and a visual directory-to-profile map is
// not.
import { api } from '../../lib/api.js';
import { banner, busy, guard, esc } from '../../lib/dom.js';
import { icon } from '../../lib/icons.js';
import { avatar } from '../../lib/avatar.js';
import { tilde } from '../../lib/paths.js';
import { state, refresh } from '../../lib/store.js';

export default {
  id: 'paths',
  label: 'Path mappings',
  icon: 'folder',

  render() {
    const scoped = state.profiles.filter((p) => p.root);
    const rows = scoped.map((p) => `
      <tr>
        <td><code class="path" title="${esc(p.root)}">${esc(tilde(p.root))}</code></td>
        <td>
          <span class="mapped">${avatar(p.name, 'sm')} ${esc(p.name)}
            ${p.active ? '<span class="badge quiet">active</span>' : ''}</span>
        </td>
        <td class="dim">${esc(p.git_email)}</td>
        <td class="right">
          <button class="ghost icon" data-act="unscope" data-name="${esc(p.name)}"
            aria-label="Clear the mapping for ${esc(p.name)}" title="Clear this mapping">${icon('x')}</button>
        </td>
      </tr>`).join('');

    // Profiles that already have a root are listed last: the common next action
    // is mapping one that has none.
    const unscoped = state.profiles.filter((p) => !p.root);
    const options = [...unscoped, ...scoped]
      .map((p) => `<option value="${esc(p.name)}">${esc(p.name)}</option>`).join('');

    return `
      <div class="page-head">
        <div>
          <h1>Path mappings</h1>
          <p>
          Repos under a mapped directory always use that profile, whatever is globally
          active. This is the feature that makes forgetting to switch harmless — git
          applies it through <code>includeIf</code>, with nothing running in the background.
          </p>
        </div>
      </div>

      <article class="card">
        ${scoped.length ? `<table>
          <thead><tr><th>Directory</th><th>Profile</th><th>Commits as</th><th></th></tr></thead>
          <tbody>${rows}</tbody>
        </table>` : '<p class="empty">No directories mapped yet.</p>'}
      </article>

      ${state.profiles.length ? `
        <div class="section-head"><h2>Map a directory</h2></div>
        <article class="card">
          <form id="map-form" class="row">
            <select name="name" aria-label="Profile">${options}</select>
            <input name="root" placeholder="~/work" required aria-label="Directory">
            <button type="submit">${icon('plus')} Map</button>
          </form>
          <p class="hint">
            Directories may not contain one another. Nested scopes would make the winning
            identity depend on config order rather than on anything you can see, so the
            server refuses them.
          </p>
        </article>` : ''}`;
  },

  actions: {
    unscope: ({ name, btn }) => guard(() => busy(btn, async () => {
      await api('PATCH', `/api/profiles/${encodeURIComponent(name)}`, { root: '' });
      banner(`Cleared the directory scope on ${name}.`);
      await refresh();
    })),
  },

  forms: {
    'map-form': ({ data, submit }) => guard(() => busy(submit, async () => {
      await api('PATCH', `/api/profiles/${encodeURIComponent(data.name)}`, { root: data.root });
      banner(`Mapped ${data.root} to ${data.name}.`);
      await refresh();
    })),
  },
};
