---
id: deleting-things
title: Deleting things
group: Rules
summary: What a deletion removes, what blocks one, and why you are asked to choose.
---

## Everything is previewed first

Nothing here can be undone, so every deletion is previewed. The preview lists the exact records that would go — "profile versions: 2", "candidate-only artifacts: 1" — rather than saying "and related data".

## Deleting a candidate is blocked by initiatives

A candidate cannot be deleted while any initiative references them, and that includes archived initiatives. An archive is a record of work that happened, and deleting its subject would leave an account of a search for nobody.

The preview names the initiatives that are blocking, so you can act on them rather than being told "cannot delete".

## Shared documents make you choose

If a document is attached to the candidate and to something else, deleting the candidate asks you to decide: remove it everywhere, or keep it under its other links.

Neither is a safe default. Deleting by default destroys evidence somebody attached on purpose; keeping by default leaves a resume in the system after its subject was deleted. So the application refuses to choose.

## What a deletion removes

Deleting a candidate removes their profile versions, their aspects, the documents attached only to them, and everything derived from those: sections, embeddings, matches.

Purging a role removes its current and historical listings, its profile and aspects, its embeddings, its matches, and any active drafts. Your notes survive with the role reference cleared, and audit events survive as metadata.

## It either all happens or none of it does

A deletion runs as one transaction. If any step fails, nothing is removed. Afterwards the application checks that what should be gone is gone, and tells you if anything survived rather than reporting success.
