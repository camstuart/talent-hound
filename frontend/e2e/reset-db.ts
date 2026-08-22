import fs from "node:fs";
import path from "node:path";

// The E2E database is emptied before every run.
//
// It used to persist, and nothing bounded its growth: every run left its
// initiatives, candidates, roles and artifacts behind for the next one. After a
// session of repeated runs it held 6,857 initiatives, 1,270 candidates and
// 1,084 roles — and the sidebar lists every initiative, the records panel lists
// every candidate, and both reload on every workspace change, in four parallel
// browser contexts.
//
// The suite did not fail honestly. It failed as one or two arbitrary specs per
// run, always "the thing I just created is not on screen within fifteen
// seconds", in whichever spec happened to lose the race — which reads as
// flakiness in the product. Measured: the same suite on the accumulated
// database took 4.3 to 5.8 minutes and failed a spec or two most runs; on an
// empty one it takes 28 seconds and passes.
//
// A suite whose reliability depends on how many times it has been run before
// cannot certify anything, and the laptop gates depend on this one.
//
// Called when the config module loads rather than from globalSetup, because
// Playwright starts the web server before globalSetup runs — so a reset there
// empties the database out from under a server that already has it open, which
// fails thirty-five specs instead of fixing one.
export function resetDatabase() {
  const dir = path.resolve(".e2e-db");
  if (!fs.existsSync(dir)) return;
  // The write-ahead log and shared-memory file are part of the database: an
  // e2e.db deleted without them reopens holding what they still describe.
  for (const name of ["e2e.db", "e2e.db-wal", "e2e.db-shm"]) {
    fs.rmSync(path.join(dir, name), { force: true });
  }
}
