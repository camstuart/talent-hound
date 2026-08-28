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

## Installing Ollama

Ollama is the program that runs the three models on your computer. Talent Hound does not include it unless your build says so on the Ollama step, so on most machines you install it once:

1. Download it from ollama.com/download and run the installer.
2. Open Ollama. It runs in the background — on Windows and macOS you will see its icon in the tray or menu bar. Nothing else needs configuring: Talent Hound looks for it at its standard address, localhost:11434.
3. In Talent Hound, press *Check again* on the Ollama step, or *Check the system again* in Settings. The step turns to done, and the next step downloads the models.

If Ollama is installed but the step still says it is not reachable, it is not running: start it from the Start menu or Applications. If it says Ollama did not answer in time, it is probably still starting — wait a moment and check again.

## Availability

Each role reports one of: ready, no model chosen, not installed, Ollama not running, download declined, download failed, no answer in time, unexpected answer, or not enough memory.

They are separate states because what you do about each is different. "Not installed" is a download. "Ollama not running" is starting Ollama. "Not enough memory" is a smaller model.

## Validated and unvalidated

A model is labelled validated only after it has passed the profile and matching benchmarks on this machine. Anything else is unvalidated — usable, but the application does not vouch for it.

The application does not try to detect a bad model at run time. It reports schema failures, timeouts, and memory errors directly instead of guessing.

## Determinism

Extraction calls run at temperature zero with a fixed seed, so the same document produces the same profile. That matters because a profile's identity is its sources and its versions — if the same inputs gave different answers, the identity would be a lie.
