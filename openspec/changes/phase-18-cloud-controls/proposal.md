## Why

Everything so far runs locally, and that is the product's promise. This phase adds one deliberate exception — a cloud endpoint the recruiter may enable for specific tasks — and the entire design problem is making that exception incapable of becoming the rule.

Three things make it safe. The first is a fixed list of what may *never* go to the cloud, enforced in code rather than by configuration: raw candidate artifacts, Candidate Profile extraction, and embeddings. There is no setting that permits them, so no misconfiguration can.

The second is consent that is narrow by construction. An approval is for one initiative, one endpoint revision, and one task type. It does not generalize, it does not persist across an endpoint change, and it can be revoked.

The third is that the recruiter sees the actual payload before the first send, and can see it before any send. Not a description of it — the bytes, exactly as Phase 14 established for search queries, and for the same reason: a preview generated separately from the request is a preview of something else.

## What Changes

- Add one optional cloud endpoint, configured per task rather than globally.
- Enforce a permanent deny list: raw candidate artifacts, Candidate Profile extraction, and candidate embeddings never leave the machine, under any configuration.
- Bind consent to the exact initiative, endpoint revision, and task type, and reset every approval when the endpoint changes.
- Show the actual payload before the first send for a task, and keep it previewable afterwards.
- Require an explicit payload selection and preview for every cloud chat send.
- Replace known structured direct identifiers with placeholders in eligible payloads.
- Create a metadata-only audit event for every non-localhost request, on the terms Phase 14 already set.

## Capabilities

### New Capabilities
- `cloud-boundary`: what may never go, and why no configuration can change it.
- `cloud-consent`: what an approval covers, what it does not, and what resets it.
- `payload-preview`: the actual payload, first use and after, and per-send for chat.
- `identifier-placeholders`: which identifiers are replaced and what survives.

### Modified Capabilities
- `disclosure-audit`: the same event, now recorded for cloud requests as well as searches, with the same prohibition on content.

## Impact

- `internal/db/migrations.go`: migration 16 — `cloud_consents`, keyed by initiative, endpoint revision, and task.
- New `internal/cloud/` — the deny list, the eligible tasks, and the placeholder substitution, all pure so the allow/deny matrix is a table.
- New `cloudservice.go` — the endpoint, consent, preview, and the refusals.
- `frontend/src/components/CloudPanel.tsx` — the endpoint, the per-task approvals with their payload previews, and revocation.
- A complete allow/deny matrix over every task; identifier fixtures; consent-scope fixtures proving one approval does not authorize another; endpoint-change and revocation fixtures.
- A fake cloud endpoint asserting it receives nothing it should not, under every configuration the tests can produce.
