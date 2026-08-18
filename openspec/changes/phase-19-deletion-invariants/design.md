## Context

Every phase before this added something. This one takes things away, permanently, from a database that holds information about people. The design question is not how to delete rows — it is how to be *sure* afterwards, and how to refuse when being sure is not possible.

## Goals / Non-Goals

**Goals:**
- Every row of the PRD's table implemented and asserted.
- One transaction per deletion, verified before success is reported.
- Refusals that name what is blocking, so the recruiter can act.
- Previews that list exactly what will go.

**Non-Goals:**
- No soft delete, no recycle bin. The PRD says delete; a bin is a second copy of the thing someone asked to remove.
- No cascade to shared records. Deleting an initiative never deletes a candidate, and this is the rule most likely to be "improved" by someone tidying up.
- No bulk delete beyond purge-all-stale, which the PRD names and which applies the same invariant per role.

## Decisions

### Verification is a query for absence, scoped to the entity

After the transaction commits, the service asks: are there chunks for this artifact, embeddings for those chunks, profiles for this subject, matches for this role? If any answer is yes, the deletion is reported failed even though it committed.

The alternative — trusting the cascade — fails silently, which is the failure mode that matters: a cascade that misses embeddings reports success, and the vectors sit there answering searches about someone who was deleted.

Scoped to the entity is the other half. A chunk shared with another role is not a failure, and a verification that counted it would make every correct deletion look broken until someone turned the check off. So the queries ask about what this entity exclusively owned.

### A blocked deletion names its blockers

Candidate deletion blocked by initiatives returns the initiatives, including archived ones, by name. Artifact deletion blocked by other links returns the links.

A refusal that says "cannot delete" and stops is a refusal the recruiter cannot act on, so they either give up or find a way around it. Naming the blockers turns the refusal into a to-do list.

### Archived is not a lesser state

An archived initiative blocks candidate deletion exactly as an active one does. The PRD says so, and the reason is that an archive is a record of work that happened — deleting its subject leaves an account of a search for nobody.

### Shared artifacts require a decision, not a default

A candidate artifact linked to something else stops the deletion until the recruiter chooses: delete it everywhere, or keep it under its other links having been told it may contain candidate information.

Neither default is safe. Deleting by default destroys evidence someone attached deliberately; keeping by default leaves a résumé in the system after its subject was deleted. So the application refuses to choose, which is the honest position.

### Exa source artifacts are role-owned and read-only

They cannot be detached and cannot be deleted individually. Purging the role takes them, current and historical.

A role's provenance is the sequence of listings it was seen as. Letting one be removed leaves a match citing a listing that no longer exists, in a system whose whole premise is that claims are checkable.

### Survivors carry cleared references, not deleted rows

A recruiter's note about a purged role survives with its role reference cleared. A CopiedOut event survives with its draft or role reference cleared.

The note is the recruiter's own words, which nothing else in the system authored, and losing it because a listing came down would be the application deleting the user's work. The audit event is a record that something left the machine, which stays true after the thing it was about is gone.

*ponytail: references are nulled in place. A tombstone table naming what used to be there is a real feature and nobody has asked for it.*

### Fault injection is a first-class test

Each cascade step is made to fail in turn, and the assertion is that nothing changed. Not "most things" — the count of every affected table is identical to before.

This is the test that justifies the transaction. Without it, "cascades run transactionally" is a claim about code nobody has exercised in the one condition that matters.

## Risks / Trade-offs

- **Verification costs a handful of queries per deletion.** Irrelevant at desktop scale, and the alternative is trusting a cascade.
- **Blocked deletions will frustrate.** A recruiter who wants a candidate gone must delete the initiatives first. That is the PRD's rule and it protects the archive.
- **Cleared references leave dangling text.** A note about "the role" whose role is gone reads oddly. It reads better than the note being gone.
- **No undo.** Deliberate. The preview is the moment to change your mind.
