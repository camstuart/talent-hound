/**
 * latestOnly wraps a panel's reload so a list can never go backwards.
 *
 * A panel has several reasons to reload — it mounted, the workspace changed,
 * the recruiter did something, a poll timer fired — and they overlap. Two
 * problems follow, and a panel that solves one usually still has the other.
 *
 * The first is order: the slower reload lands last and puts the list back the
 * way it was before the record the recruiter just added. So only the newest
 * call may write, which is what `isCurrent` answers.
 *
 * The second is failure, and it is the one that survives fixing the first. The
 * older reload that would have written was discarded the moment the newer one
 * started, so if the newer one fails, nothing writes at all: the list keeps
 * showing a workspace the database has already moved past, with the record
 * sitting in it. The recruiter is told their entry failed, it did not, and
 * entering it again makes a duplicate. Measured under a parallel E2E run, where
 * a dropped response is ordinary — the company was row 164 in the database and
 * absent from the screen.
 *
 * So a failed reload retries once, and only if it is still the newest: a burst
 * of reloads makes one retry between them, not one each. If the retry fails too
 * the error is thrown, because by then it is not transient and the caller
 * should say so.
 */
export function latestOnly(load: (isCurrent: () => boolean) => Promise<void>): () => Promise<void> {
  let latest = 0;
  return async function reload(retried = false): Promise<void> {
    const mine = ++latest;
    try {
      await load(() => mine === latest);
    } catch (err) {
      // A superseded reload's failure is nobody's problem: a newer one is
      // already on its way with the answer.
      if (mine !== latest) return;
      if (retried) throw err;
      return reload(true);
    }
  };
}
