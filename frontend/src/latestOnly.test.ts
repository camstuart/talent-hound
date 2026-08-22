import { describe, it, expect } from "vitest";
import { latestOnly } from "./latestOnly";

const defer = () => {
  let resolve!: (v?: unknown) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise((res, rej) => {
    resolve = res as (v?: unknown) => void;
    reject = rej;
  });
  return { promise, resolve, reject };
};

describe("latestOnly", () => {
  it("lets an older reload finish without writing over a newer one", async () => {
    const first = defer();
    const second = defer();
    const gates = [first, second];
    const written: string[] = [];
    let n = 0;
    const reload = latestOnly(async (isCurrent) => {
      const mine = n++;
      await gates[mine].promise;
      if (!isCurrent()) return;
      written.push(mine === 0 ? "first" : "second");
    });

    const a = reload();
    const b = reload();
    // The newer one lands first, then the older one arrives late.
    second.resolve();
    await b;
    first.resolve();
    await a;

    expect(written).toEqual(["second"]);
  });

  it("retries once when the newest reload fails", async () => {
    let calls = 0;
    const written: string[] = [];
    const reload = latestOnly(async (isCurrent) => {
      calls += 1;
      if (calls === 1) throw new Error("the connection was reset");
      if (isCurrent()) written.push("ok");
    });

    await reload();

    expect(calls).toBe(2);
    expect(written).toEqual(["ok"]);
  });

  it("gives up after one retry rather than retrying forever", async () => {
    let calls = 0;
    const reload = latestOnly(async () => {
      calls += 1;
      throw new Error("the connection was reset");
    });

    await expect(reload()).rejects.toThrow("the connection was reset");
    expect(calls).toBe(2);
  });

  it("does not retry a reload another has already superseded", async () => {
    const gate = defer();
    let calls = 0;
    const reload = latestOnly(async () => {
      calls += 1;
      if (calls === 1) {
        await gate.promise;
        throw new Error("the connection was reset");
      }
    });

    const stale = reload();
    await reload(); // supersedes it
    gate.resolve();
    await stale;

    // Two calls: the superseded one and the one that replaced it. The
    // superseded failure is not retried, because a newer answer already landed.
    expect(calls).toBe(2);
  });
});
