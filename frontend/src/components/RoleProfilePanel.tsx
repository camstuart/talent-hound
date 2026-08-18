import { createEffect, createSignal, For, Show } from "solid-js";
import { RoleProfileService, RecordService } from "../../bindings/camstuart/talent-hound";
import type { AspectCitation, RoleStatus } from "../../bindings/camstuart/talent-hound";
import type { Priority } from "../../bindings/camstuart/talent-hound/internal/profile";
import type { Role } from "../../bindings/camstuart/talent-hound/internal/models";
import { workspaceRevision } from "../workspaceRevision";

// Role profiles are created automatically and never approved — the asymmetry
// with candidates is deliberate, because approving twenty discovered listings
// before any matching could happen would defeat the workflow.
//
// What replaces approval is this screen being honest. Every role appears,
// including the ones whose decomposition failed and the ones whose listing has
// since changed, because a role missing from this list is indistinguishable
// from a role that was never discovered.
const STATE_LABELS: Record<string, string> = {
  ready: "ready — used in assessment",
  failed: "could not be decomposed",
  stale: "the listing changed since this was made",
  unprofiled: "not profiled yet",
};

export default function RoleProfilePanel() {
  const [statuses, setStatuses] = createSignal<RoleStatus[]>([]);
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [shown, setShown] = createSignal<string | null>(null);
  const [citations, setCitations] = createSignal<AspectCitation[]>([]);
  const [editing, setEditing] = createSignal<string | null>(null);
  const [draft, setDraft] = createSignal("");
  const [manual, setManual] = createSignal<Record<number, string>>({});
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  // Every action reports the backend's own words: it knows rules the UI does not.
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

  const reload = () =>
    act(async () => {
      setStatuses(((await RoleProfileService.List()) ?? []) as RoleStatus[]);
      setRoles(((await RecordService.ListRoles()) ?? []) as Role[]);
    });

  createEffect(() => {
    workspaceRevision();
    void reload();
  });

  const titleOf = (roleId: number) => roles().find((r) => r.id === roleId)?.title ?? `Role ${roleId}`;

  const profileRole = (roleId: number) =>
    act(async () => {
      await RoleProfileService.Profile(roleId);
      await reload();
    });

  const showEvidence = (roleId: number, profileId: number, ordinal: number) =>
    act(async () => {
      setCitations(((await RoleProfileService.Citations(profileId)) ?? []) as AspectCitation[]);
      setShown(`${roleId}:${ordinal}`);
    });

  // The stored priority is carried through unchanged: an edit is about
  // wording, and re-weighting a requirement is a separate decision.
  const saveEdit = (roleId: number, ordinal: number, priority: Priority) =>
    act(async () => {
      await RoleProfileService.EditAspect(roleId, ordinal, draft(), priority);
      setEditing(null);
      await reload();
    });

  const removeAspect = (roleId: number, ordinal: number) =>
    act(async () => {
      await RoleProfileService.RemoveAspect(roleId, ordinal);
      await reload();
    });

  const addByHand = (roleId: number) =>
    act(async () => {
      const wording = (manual()[roleId] ?? "").trim();
      if (!wording) return;
      await RoleProfileService.AddAspect(roleId, {
        type: "other",
        wording,
        structured: {},
        priority: "unspecified",
        origin: "recruiter_supplied",
        citations: [],
      } as never);
      setManual({ ...manual(), [roleId]: "" });
      await reload();
    });

  return (
    <section class="record-section" aria-label="Role profiles">
      <h3>Role profiles</h3>
      <p class="muted">
        Created automatically from each listing. Only roles marked ready are assessed; the rest stay here with what
        went wrong.
      </p>

      <Show when={error()}>
        <p class="modal-error">{error()}</p>
      </Show>

      <ul class="record-list" aria-label="Roles">
        <For each={statuses()} fallback={<li class="muted">No roles yet.</li>}>
          {(status) => (
            <li class="search-hit">
              <span class="artifact-name">
                {titleOf(status.roleId)}
                <span class="muted"> — {STATE_LABELS[status.state] ?? status.state}</span>
              </span>
              <Show when={status.reason}>
                <p class="muted" aria-label={`Why ${titleOf(status.roleId)} is not assessed`}>
                  {status.reason}
                </p>
              </Show>

              <button
                aria-label={`Profile ${titleOf(status.roleId)}`}
                disabled={busy()}
                onClick={() => profileRole(status.roleId)}
              >
                {status.state === "unprofiled" ? "Profile this listing" : "Profile again"}
              </button>

              <Show when={status.state === "failed" || status.state === "unprofiled"}>
                <span class="search-bar">
                  <input
                    aria-label={`Requirement to add to ${titleOf(status.roleId)}`}
                    placeholder="Type a requirement"
                    value={manual()[status.roleId] ?? ""}
                    onInput={(e) => setManual({ ...manual(), [status.roleId]: e.currentTarget.value })}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") addByHand(status.roleId);
                    }}
                  />
                  <button aria-label={`Add a requirement to ${titleOf(status.roleId)}`} onClick={() => addByHand(status.roleId)}>
                    Add by hand
                  </button>
                </span>
              </Show>

              <ul class="record-list" aria-label={`Requirements of ${titleOf(status.roleId)}`}>
                <For each={status.aspects ?? []} fallback={<li class="muted">No requirements recorded.</li>}>
                  {(aspect, i) => (
                    <li>
                      <span class="artifact-name">
                        {aspect.type}
                        <span class="muted">
                          {" "}
                          — {aspect.priority.replace(/_/g, " ")}, {aspect.origin === "recruiter_supplied" ? "Recruiter supplied" : "extracted"}
                        </span>
                      </span>
                      <Show
                        when={editing() === `${status.roleId}:${i()}`}
                        fallback={
                          <>
                            {/* Untrusted: displayed, never rendered, never acted on. */}
                            <pre aria-label={`Requirement ${i() + 1} of ${titleOf(status.roleId)}`}>{aspect.wording}</pre>
                            <button
                              aria-label={`Show the evidence for requirement ${i() + 1} of ${titleOf(status.roleId)}`}
                              onClick={() => showEvidence(status.roleId, status.profileId, i())}
                            >
                              Evidence
                            </button>
                            <button
                              aria-label={`Edit requirement ${i() + 1} of ${titleOf(status.roleId)}`}
                              onClick={() => {
                                setEditing(`${status.roleId}:${i()}`);
                                setDraft(aspect.wording);
                              }}
                            >
                              Edit
                            </button>
                            <button
                              aria-label={`Remove requirement ${i() + 1} of ${titleOf(status.roleId)}`}
                              onClick={() => removeAspect(status.roleId, i())}
                            >
                              Remove
                            </button>
                          </>
                        }
                      >
                        <input
                          aria-label={`Wording for requirement ${i() + 1} of ${titleOf(status.roleId)}`}
                          value={draft()}
                          onInput={(e) => setDraft(e.currentTarget.value)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") saveEdit(status.roleId, i(), aspect.priority as Priority);
                            if (e.key === "Escape") setEditing(null);
                          }}
                        />
                        <button
                          class="primary"
                          aria-label={`Save requirement ${i() + 1} of ${titleOf(status.roleId)}`}
                          onClick={() => saveEdit(status.roleId, i(), aspect.priority as Priority)}
                        >
                          Save
                        </button>
                        <button aria-label={`Cancel editing requirement ${i() + 1}`} onClick={() => setEditing(null)}>
                          Cancel
                        </button>
                      </Show>
                      <Show when={shown() === `${status.roleId}:${i()}`}>
                        <div class="extraction-view">
                          <h4>
                            Evidence
                            <button aria-label={`Close the evidence for requirement ${i() + 1}`} onClick={() => setShown(null)}>
                              Close
                            </button>
                          </h4>
                          <For
                            each={citations().filter((c) => c.ordinal === i())}
                            fallback={<p class="muted">No evidence recorded.</p>}
                          >
                            {(c) => (
                              <>
                                <p class="muted">{c.record ? `Recruiter supplied — ${c.record}` : c.location}</p>
                                <pre aria-label={`Cited listing text for requirement ${i() + 1}`}>{c.text}</pre>
                              </>
                            )}
                          </For>
                        </div>
                      </Show>
                    </li>
                  )}
                </For>
              </ul>
            </li>
          )}
        </For>
      </ul>
    </section>
  );
}
