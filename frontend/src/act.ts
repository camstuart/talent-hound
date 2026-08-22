import { createSignal } from "solid-js";
import { latestOnly } from "./latestOnly";

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

  /**
   * refresh is act for a reload the recruiter did not ask for.
   *
   * It reports failures the same way and deliberately does not mark the panel
   * busy. Panels disable their controls while busy, so routing a background
   * reload through act disabled every button for as long as it ran — and a
   * click that lands in that window is dropped silently, with nothing to say it
   * was. A refresh is not an action, and it should not take the controls away.
   */
  const refresh = async (run: () => Promise<unknown>) => {
    try {
      await run();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  /**
   * reloader wires a panel's list refresh: guarded so only the newest may
   * write and a failed one retries, reported if it fails for good, and not
   * marked busy because the recruiter did not ask for it.
   *
   * All three belong together — a panel that took them one at a time is how
   * the list came to show a record the database already held.
   */
  const reloader = (load: (isCurrent: () => boolean) => Promise<void>) => {
    const guarded = latestOnly(load);
    return () => refresh(guarded);
  };

  return { act, refresh, reloader, error, busy, setError };
}
