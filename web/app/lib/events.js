import { TOKEN } from './api.js';

// subscribe keeps the page correct when the identity is switched from a
// terminal. EventSource cannot set headers, hence the token in the query string
// — the same one the page was opened with, over loopback only.
//
// Reconnection is left to EventSource, which retries on its own; the server
// sends a keepalive comment every 30s so nothing in the stack reaps an idle
// connection first.
export function subscribe(onProfiles, onDead) {
  const events = new EventSource(`/api/events?t=${encodeURIComponent(TOKEN)}`);
  events.addEventListener('profiles', onProfiles);

  // A transient drop leaves the stream retrying, and saying anything about that
  // would be noise. A stream in CLOSED never comes back: the server refused the
  // token outright, which means this tab is holding one from a previous run.
  events.addEventListener('error', () => {
    if (events.readyState === EventSource.CLOSED) onDead?.();
  });

  return events;
}
