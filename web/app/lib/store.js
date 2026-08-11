import { api } from './api.js';

// The whole client state. It is small enough to keep in one object, and keeping
// it in one object is what lets any feature call refresh() and have every other
// screen come back correct.
//
// Nothing here imports a feature: features depend on the store, the store
// notifies whoever is listening, and main.js is the only thing that knows both.
export const state = {
  profiles: [],
  active: '',
  home: '',
  view: 'profiles',
  editing: null,   // profile name whose edit form is open
  adding: false,   // the add-account form is disclosed
  keys: {},        // name -> public key, revealed on demand
  doctor: null,    // last doctor result, so switching tabs does not discard it
  offline: false,  // doctor's "skip network checks" box
  loaded: false,
};

let listener = () => {};

export function onChange(fn) { listener = fn; }
export function notify() { listener(); }

export async function refresh() {
  const data = await api('GET', '/api/profiles');
  state.profiles = data.profiles;
  state.active = data.active;
  state.home = data.home || '';
  state.loaded = true;
  notify();
}
