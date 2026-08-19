---
id: getting-started
title: Getting started
group: First steps
summary: What Talent Hound is, what it needs, and what the first run asks you.
---

## What this application is

Talent Hound is a local-first recruiting workspace. Everything you load — documents, candidates, companies, roles, profiles, notes — stays in one folder on this machine. Models run locally through Ollama. Nothing about a candidate is sent anywhere because you added it.

It helps you do four things: keep evidence in order, turn it into structured profiles you approve, find and rank roles against a candidate, and write drafts you copy out yourself.

## What it needs

- **A data folder on an encrypted volume.** Real candidate data is only available when the volume holding your data folder is encrypted. On Windows that means BitLocker or Device Encryption.
- **Ollama**, running locally, with three models: one to embed, one to classify, one to generate. Together they are roughly 4–8 GB.
- **The document reader**, which ships with the application and converts PDFs and Word files to text.

## The first run

First run asks you, in this order: choose the data folder, verify the volume is encrypted, verify the document reader, verify Ollama, install the required models, acknowledge how the data will be handled, and create your first initiative.

Each step blocks the ones after it, and none of them is remembered as "done" — the application checks what is true each time it asks. If you uninstall Ollama a month later, setup will say so again.

## If a step will not pass

The step names what is missing rather than saying setup failed. A missing document reader names the path it looked in. An unreachable Ollama names the address it tried. A volume that is not encrypted says so, and a volume that could not be checked says *that*, which is a different problem with a different fix.
