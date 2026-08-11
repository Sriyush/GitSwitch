// The doctor screen: per-check pass/fail/skip with the remedy inline.
//
// Skipped is rendered distinctly from passed on purpose — the checks that need
// internal/keyring and internal/github verify nothing yet, and showing them as
// green would be a lie about what has been proven.
import { api } from '../../lib/api.js';
import { $, banner, busy, guard, esc } from '../../lib/dom.js';
import { icon } from '../../lib/icons.js';
import { avatar } from '../../lib/avatar.js';
import { foldHome } from '../../lib/paths.js';
import { state } from '../../lib/store.js';

export default {
  id: 'doctor',
  label: 'Doctor',
  icon: 'stethoscope',

  render() {
    return `
      <div class="page-head">
        <div>
          <h1>Doctor</h1>
          <p>Every check that can be made locally, plus an SSH handshake per account.</p>
        </div>
        <div class="actions">
          <label class="check"><input type="checkbox" id="offline"${state.offline ? ' checked' : ''}> Skip network checks</label>
          <button data-act="run-doctor">Run checks</button>
        </div>
      </div>
      <div id="doctor-out">${state.doctor ? results(state.doctor) : '<p class="empty">Not run yet.</p>'}</div>`;
  },

  // Doctor with nothing in it is a dead end, so it runs on arrival. Later
  // visits reuse the stored result rather than re-running an SSH handshake
  // every time the tab is clicked.
  onEnter() {
    if (!state.doctor) guard(() => run($('button[data-act="run-doctor"]')));
  },

  actions: {
    'run-doctor': ({ btn }) => guard(() => run(btn)),
  },

  forms: {},
};

async function run(btn) {
  state.offline = $('#offline').checked;
  $('#doctor-out').innerHTML = '<article class="card skeleton"><span></span><span></span><span></span></article>';
  await busy(btn, async () => {
    try {
      state.doctor = await api('GET', `/api/doctor?offline=${state.offline ? '1' : '0'}`);
      $('#doctor-out').innerHTML = results(state.doctor);
    } catch (err) {
      $('#doctor-out').innerHTML = '<p class="empty">Not run yet.</p>';
      banner(err.message, 'error');
    }
  });
}

function results(res) {
  const tally = { pass: 0, fail: 0, skip: 0 };
  const count = (list) => list.forEach((c) => { tally[c.status]++; });
  res.profiles.forEach((r) => count(r.checks));
  count(res.global);

  const bad = tally.fail > 0;
  return `
    <article class="card">
      <div class="card-head">
        <div class="verdict ${bad ? 'bad' : 'ok'}">
          <span class="ring">${icon(bad ? 'alert' : 'check')}</span>
          <div>
            <h2>${bad
              ? `${tally.fail} check${tally.fail > 1 ? 's need' : ' needs'} attention`
              : 'Everything checks out'}</h2>
            <p>${tally.skip} of ${tally.pass + tally.fail + tally.skip} checks could not be run yet.</p>
          </div>
        </div>
        <div class="tally">
          <span class="pill on-ok">${icon('check')} ${tally.pass}</span>
          <span class="pill ${tally.fail ? 'on-bad' : ''}">${icon('cross')} ${tally.fail}</span>
          <span class="pill on-skip">${icon('minus')} ${tally.skip}</span>
        </div>
      </div>
    </article>

    <div class="section-head"><h2>Per account</h2></div>
    <div class="stack">
      ${res.profiles.map((r) => `
        <article class="card group">
          <div class="group-head">
            ${avatar(r.profile, 'sm')}
            <h3>${esc(r.profile)}</h3>
            <span class="dim">${esc(r.username)}</span>
            ${r.checks.some((c) => c.status === 'fail') ? '<span class="pill on-bad">needs attention</span>' : ''}
          </div>
          <ul class="checks">${checks(r.checks)}</ul>
        </article>`).join('')}
    </div>

    <div class="section-head"><h2>Global</h2></div>
    <article class="card"><ul class="checks">${checks(res.global)}</ul></article>

    <p class="hint">
      Skipped checks are not passes — they are the parts that need
      <code>internal/keyring</code> and <code>internal/github</code>, which do not exist yet.
    </p>`;
}

const MARK = { pass: 'check', fail: 'cross', skip: 'minus' };

function checks(list) {
  return list.map((c) => `
    <li class="${c.status}">
      <span class="mark" title="${esc(c.status)}">${icon(MARK[c.status])}</span>
      <span class="what">
        <b>${esc(c.label)}</b> <span class="detail">— ${esc(foldHome(c.detail))}</span>
        ${c.remedy ? `<span class="remedy">${esc(foldHome(c.remedy))}</span>` : ''}
      </span>
    </li>`).join('');
}
