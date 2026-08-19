---
id: troubleshooting
title: When something will not work
group: Setup
summary: The refusals you are most likely to meet, and what each one means.
---

## "This installation cannot hold candidate data"

The volume holding your data folder is not encrypted, or the check could not be made. Real candidate data needs an encrypted volume. Demo scope works anywhere but refuses documents and candidates, and is not a place to do real work.

If the message says the state "could not be checked", that is not the same as "not encrypted" — the application refuses to treat an unknown answer as a safe one.

## "The classify model did not answer"

The model took longer than its budget, or Ollama stopped. A large model on a CPU-only machine can take minutes on a long document. Check Ollama is running, and consider a smaller model if this is routine.

## "The classifier's output did not satisfy the contract"

The model produced statements the contract refuses — usually a quote that does not appear in the source, or the same statement twice. One repair is attempted automatically. A second failure is reported rather than retried in a loop.

You can still enter the aspects by hand, change the model, or fix the source document.

## "Nothing is embedded yet"

Semantic search and the shortlist's meaning half need an embedding space. Index the initiative first. Word search works without it.

## A criterion was refused

It named a protected attribute. The message says which category. Rewrite it in terms of what the work requires — "must have existing work rights" rather than a nationality.

## A search found nothing

The application says so plainly rather than showing an empty list as though it were a result. A live search that finds very few roles is reported as inconclusive coverage rather than as a verdict about the candidate.

## The interface says a profile is stale

The sources changed after the profile was built. It still works. Rebuild it when you want it to reflect the current documents.
