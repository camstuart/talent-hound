---
id: models
title: Models
group: Setup
summary: The three roles, what each does, and what to do when one is unavailable.
---

## Three roles

- **Embed** turns text into vectors so meaning can be compared. Used for semantic search and for matching.
- **Classify** decomposes documents into typed, cited statements. Used for profiles and for proposing criteria.
- **Generate** writes assessments, answers, and drafts.

You assign a model to each role. Generate can stand in for classify if you have not assigned one separately.

## Availability

Each role reports one of: ready, no model chosen, not installed, Ollama not running, download declined, download failed, no answer in time, unexpected answer, or not enough memory.

They are separate states because what you do about each is different. "Not installed" is a download. "Ollama not running" is starting Ollama. "Not enough memory" is a smaller model.

## Validated and unvalidated

A model is labelled validated only after it has passed the profile and matching benchmarks on this machine. Anything else is unvalidated — usable, but the application does not vouch for it.

The application does not try to detect a bad model at run time. It reports schema failures, timeouts, and memory errors directly instead of guessing.

## Determinism

Extraction calls run at temperature zero with a fixed seed, so the same document produces the same profile. That matters because a profile's identity is its sources and its versions — if the same inputs gave different answers, the identity would be a lie.
