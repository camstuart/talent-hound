import { createSignal } from "solid-js";

// One counter the whole workspace watches. A panel that changes something other
// panels display bumps it; those panels reload when it moves.
//
// This is deliberately not an event bus with typed messages. There is one kind
// of news — "something in this workspace changed" — and the panels already know
// how to reload themselves, so a number they can watch is the entire mechanism.
//
// ponytail: one global counter, everyone reloads. Split it per entity when a
// panel's reload becomes expensive enough that reloading it needlessly shows.
const [revision, setRevision] = createSignal(0);

/** workspaceRevision is read inside a panel's effect to subscribe to changes. */
export const workspaceRevision = revision;

/** bumpWorkspace tells every watching panel that something changed. */
export function bumpWorkspace() {
  setRevision((n) => n + 1);
}
