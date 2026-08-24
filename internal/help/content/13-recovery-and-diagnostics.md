---
id: recovery-and-diagnostics
title: Backups, recovery, and diagnostics
group: Setup
summary: Copying your data safely, moving to a new machine, and what the diagnostics show.
---

## Copying your data

Close Talent Hound completely, then copy the whole data folder. That folder holds everything: records, documents, profiles, indexes, audit events, job state, migration snapshots, and logs.

Two things are not in it, by design: provider keys in Windows Credential Manager or macOS Keychain, and models in Ollama's storage. Both are re-entered or re-downloaded rather than copied.

## Recovering onto another machine

1. Install Talent Hound and Ollama.
2. Start Talent Hound and select the copied folder.
3. The application checks the folder can be written to, that it holds a database, and that the database passes an integrity check.
4. Before any schema migration it takes a snapshot inside the folder.
5. If a migration fails, the snapshot is restored and the folder is not opened.
6. Re-enter your provider keys and re-download any missing models.

A failed check never opens a partially recovered database and never overwrites your only copy. A folder that cannot be written to is refused before anything is read, because finding that out during a migration is how a copy gets lost.

## Diagnostics

The diagnostics view shows the application version, the schema version, your folder paths, whether the volume is encrypted, whether the reader and Ollama are available, which models are assigned, how many records of each kind exist, and recent job outcomes as codes.

It contains nothing about any candidate. It is safe to show to somebody helping you.

## Deleting everything

The delete-all action names the exact folder it will empty and asks you to type that folder back. It does not accept "yes".
