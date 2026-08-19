---
id: discovery
title: Finding roles
group: Working
summary: What a search sends, what it strips, and how a role goes stale.
---

## You see the query before it is sent

A role search shows you the exact text that will be sent. Not a summary of it — the bytes. The query you approve is the query that goes.

## What is removed

Names, preferred names, email addresses, phone numbers, and street addresses are removed and replaced. So is the current employer's name, because naming it can identify a person as surely as their own name does.

What survives is the professional shape: "senior platform engineer, Go and SQLite, Melbourne, permanent". That is what makes it a search rather than a disclosure.

The application warns you when a query still contains something that looks like an organisation, and warns more strongly when it looks like an identifier.

## What is recorded

Every search is recorded as an event: when, which provider, which initiative, and what kinds of information were disclosed — "professional requirements". The record never contains the query text or the results.

## Sources and staleness

A role discovered by a search keeps the listing it came from, as an artifact you can read. If the listing changes, the role's profile is stale and says so. Historical listings are kept, so you can see what the role said when you shortlisted it.

A stale role is excluded from new shortlists until it is refreshed, because ranking a candidate against a listing that no longer exists wastes your time.
