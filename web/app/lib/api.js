// The JSON API client, and the one place the session token lives.
//
// The token arrives in the query string because that is the only channel to a
// browser that has not run any of our code yet. It is stripped from the URL
// immediately — it should not sit in the address bar, get copied into a chat
// window, or survive in history — and kept in sessionStorage instead.
//
// sessionStorage, specifically, and never localStorage: it is scoped to this one
// tab and is cleared when the tab closes, so a reload keeps working while a dead
// credential cannot outlive the window that was given it. localStorage would
// leave the token lying in the browser profile long after the process it
// authenticates against had exited.
const STORE_KEY = 'gitswitch.token';

// Storage throws rather than returns null in some privacy configurations, and a
// UI that cannot survive a reload is better than a UI that will not load.
function stored(value) {
  try {
    if (value === undefined) return sessionStorage.getItem(STORE_KEY) || '';
    if (value === null) sessionStorage.removeItem(STORE_KEY);
    else sessionStorage.setItem(STORE_KEY, value);
  } catch { /* fall through to the in-memory copy */ }
  return value || '';
}

const fromURL = new URLSearchParams(location.search).get('t') || '';
if (fromURL) {
  // A token in the URL is always the freshest one: it came from the `gsw ui`
  // that is running right now, which may not be the one this tab last talked to.
  stored(fromURL);
  history.replaceState(null, '', location.pathname);
}

export const TOKEN = fromURL || stored();

// Every request that comes back 401 means the token this tab holds is not the
// one the running server minted — almost always because `gsw ui` was restarted.
// Dropping it stops a stale credential from being retried forever, and lets the
// page say the one useful thing: open the new URL.
export const STALE =
  'This tab\'s session token is no longer valid — `gsw ui` has been restarted since ' +
  'the page was opened. Tokens are minted per run and never stored on disk. Open the ' +
  'URL the running `gsw ui` printed.';

export async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: {
      'Authorization': 'Bearer ' + TOKEN,
      ...(body ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    stored(null);
    throw new Error(STALE);
  }
  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}
