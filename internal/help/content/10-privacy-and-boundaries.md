---
id: privacy-and-boundaries
title: What never leaves this machine
group: Rules
summary: The boundaries that are enforced in code rather than by configuration.
---

## The local promise

Everything you load stays in your data folder. Adding a document, a candidate, or a note sends nothing anywhere.

Three things can leave, each only when you say so: a role search query (previewed exactly), a cloud request for a task you approved (previewed exactly), and a draft you copy out yourself.

## What can never go to a cloud model

Even with a cloud endpoint configured and approved, three things are refused by the code and cannot be permitted by any setting:

- raw candidate documents,
- candidate profile extraction,
- embeddings.

The refusal cites the boundary rather than a missing approval, because no approval exists that would change it.

## Cloud approvals are narrow

An approval covers one initiative, one endpoint configuration, and one task. It does not generalise to another task or another initiative, and changing the endpoint invalidates every approval made against the old one.

Before the first send for a task, you see the payload — the bytes. For chat, you see it before every send, because a chat payload is different every time.

## Credentials

Provider keys are stored in the Windows credential store and nowhere else. They are not in the database, the logs, the diagnostics, or a copied data folder. There is no way to read one back through the application — a stored key can be replaced or removed, not displayed.

## Diagnostics

The diagnostic report contains versions, folder paths, dependency availability, record counts, and job outcome codes. It contains no candidate details, no queries, no payloads, no draft text, and no filenames. It is built from facts the application already holds rather than by reading your data and removing the sensitive parts.
