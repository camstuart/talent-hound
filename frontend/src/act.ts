import { createSignal } from "solid-js";

/**
 * createAction gives a panel the one error-handling shape every panel needs:
 * run something, and if the backend refuses, show the backend's own words.
 *
 * It exists because every panel had the same twelve lines, and twelve copied
 * lines are twelve chances for one of them to start swallowing errors quietly.
 * The backend knows rules the UI does not — which criterion is unlawful, which
 * profile is unapproved — so its message is the message, verbatim.
 */
export function createAction() {
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const act = async (run: () => Promise<unknown>) => {
    setError("");
    setBusy(true);
    try {
      await run();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return { act, error, busy, setError };
}
