import { esc } from '../../lib/dom.js';
import { icon } from '../../lib/icons.js';
import { avatar } from '../../lib/avatar.js';
import { state } from '../../lib/store.js';

// The add form is behind a disclosure rather than always open: on a screen whose
// job is showing which account is active, a permanently expanded seven-field
// form is the loudest thing on the page and the least often used.
export function addForm() {
  if (!state.adding) return '';
  return `
    <div class="section-head"><h2>Add an account</h2></div>
    <article class="card">
      <form id="add-form" class="grid">
        <label>Profile name<input name="name" required autofocus placeholder="work" pattern="[a-z0-9][a-z0-9._-]*"></label>
        <label>GitHub login<input name="username" required placeholder="you-acme"></label>
        <label>Commit name<input name="git_name" placeholder="defaults to the login"></label>
        <label>Commit email<input name="git_email" required type="email" placeholder="you@acme.com"></label>
        <label>Directory<input name="root" placeholder="~/work"></label>
        <label>Owners<input name="orgs" placeholder="acme, acme-labs"></label>
        <div class="form-foot">
          <label class="check"><input type="checkbox" name="generate_key" checked> Generate an SSH key</label>
        </div>
        <div class="form-foot">
          <button type="submit">Add profile</button>
          <button type="button" class="ghost" data-act="add-close">Cancel</button>
        </div>
      </form>
      <p class="hint">
        GitHub rejects a public key already registered to another account, so each
        profile needs its own. The key is generated locally; you register it yourself.
      </p>
    </article>`;
}

// The edit form posts a PATCH with every field present, which is why an empty
// input clears a value rather than leaving it alone: the server distinguishes
// "absent" from "empty", and this form is never absent.
export function editForm(p) {
  return `
    <article class="card">
      <div class="card-head">
        <div class="who-block">
          ${avatar(p.name)}
          <div class="names"><h2>Editing ${esc(p.name)}</h2>
          <p>Empty clears a value.</p></div>
        </div>
      </div>
      <form class="grid edit-form" data-name="${esc(p.name)}" style="margin-top:16px">
        <label>GitHub login<input name="username" value="${esc(p.username)}"></label>
        <label>Commit name<input name="git_name" value="${esc(p.git_name)}"></label>
        <label>Commit email<input name="git_email" type="email" value="${esc(p.git_email)}"></label>
        <label>SSH key<input name="ssh_key" value="${esc(p.ssh_key || '')}"></label>
        <label>Signing key<input name="signing_key" value="${esc(p.signing_key || '')}"></label>
        <label>Directory<input name="root" value="${esc(p.root || '')}" placeholder="empty clears the scope"></label>
        <label>Owners<input name="orgs" value="${esc((p.orgs || []).join(', '))}"></label>
        <div class="form-foot">
          <button type="submit">Save</button>
          <button type="button" class="ghost" data-act="cancel">Cancel</button>
          <span class="hint inline">Esc also cancels.</span>
        </div>
      </form>
    </article>`;
}
