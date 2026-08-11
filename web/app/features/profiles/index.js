// The profiles screen: list, switch, add, edit, remove, and the public key.
import { api } from '../../lib/api.js';
import { banner, busy, guard, esc } from '../../lib/dom.js';
import { icon } from '../../lib/icons.js';
import { tilde } from '../../lib/paths.js';
import { state, refresh, notify } from '../../lib/store.js';
import { profileCard } from './card.js';
import { addForm, editForm } from './forms.js';

export default {
  id: 'profiles',
  label: 'Accounts',
  icon: 'user',

  render() {
    const card = (p, opts) => (state.editing === p.name ? editForm(p) : profileCard(p, opts));
    const active = state.profiles.find((p) => p.active);
    const others = state.profiles.filter((p) => !p.active);

    return `
      <div class="page-head">
        <div>
          <h1>Accounts</h1>
          <p>One profile is active globally. Directory-scoped profiles override it inside their own trees.</p>
        </div>
        ${state.adding ? '' : `<button class="ghost" data-act="add-open">${icon('plus')} Add an account</button>`}
      </div>
      ${active ? card(active, { hero: true }) : '<p class="empty">No active profile yet.</p>'}
      ${others.length ? `
        <div class="section-head"><h2>Switch to</h2></div>
        <div class="stack">${others.map((p) => card(p)).join('')}</div>` : ''}
      ${state.profiles.length ? '' : '<p class="empty">No profiles yet. Add one below.</p>'}
      ${addForm()}`;
  },

  actions: {
    switch: ({ name, btn }) => guard(() => busy(btn, async () => {
      await api('POST', '/api/switch', { name });
      banner(`Switched to ${name}.`);
      await refresh();
    })),

    edit: ({ name }) => { state.editing = name; notify(); },
    cancel: () => { state.editing = null; notify(); },
    'add-open': () => { state.adding = true; notify(); },
    'add-close': () => { state.adding = false; notify(); },

    key: ({ name, btn }) => {
      if (state.keys[name]) { delete state.keys[name]; notify(); return; }
      return guard(() => busy(btn, async () => {
        const d = await api('GET', `/api/profiles/${encodeURIComponent(name)}/key`);
        state.keys[name] = d.public_key;
        notify();
      }));
    },

    // Clipboard access is origin-gated, and http://127.0.0.1 counts as a secure
    // context — but a browser that refuses still has to leave the user with the
    // key, so fall back to selecting it.
    copy: ({ name, btn }) => navigator.clipboard.writeText(state.keys[name]).then(
      () => banner(`Copied ${name}'s public key.`),
      () => {
        getSelection().selectAllChildren(btn.closest('.keybox').querySelector('pre.key'));
        banner('Clipboard blocked by the browser — the key is selected, press Ctrl-C.', 'error');
      },
    ),

    delete: ({ name, btn }) => {
      if (!confirm(`Remove profile "${name}"?\n\nThe SSH key on disk is kept.`)) return;
      return guard(() => busy(btn, async () => {
        const d = await api('DELETE', `/api/profiles/${encodeURIComponent(name)}`);
        banner(d.kept_ssh_key
          ? `Removed ${name}. Its SSH key was kept at ${tilde(d.kept_ssh_key)}.`
          : `Removed ${name}.`);
        await refresh();
      }));
    },
  },

  forms: {
    'add-form': ({ data, orgs, submit }) => guard(() => busy(submit, async () => {
      await api('POST', '/api/profiles', {
        name: data.name,
        username: data.username,
        git_name: data.git_name || data.username,
        git_email: data.git_email,
        root: data.root || '',
        orgs,
        generate_key: !!data.generate_key,
      });
      state.adding = false;
      banner(`Added ${esc(data.name)}.`);
      await refresh();
    })),

    'edit-form': ({ form, data, orgs, submit }) => guard(() => busy(submit, async () => {
      await api('PATCH', `/api/profiles/${encodeURIComponent(form.dataset.name)}`, {
        username: data.username,
        git_name: data.git_name,
        git_email: data.git_email,
        ssh_key: data.ssh_key,
        signing_key: data.signing_key,
        root: data.root,
        orgs,
      });
      state.editing = null;
      banner('Saved.');
      await refresh();
    })),
  },
};
