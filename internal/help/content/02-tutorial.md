---
id: tutorial
title: Tutorial — a candidate from document to draft
group: First steps
summary: The whole flagship loop in order, with what to expect at each step.
---

## 1. Create an initiative

An initiative is one piece of work: finding roles for a candidate, finding candidates for a role, or business development. Everything you do happens inside one, and records like candidates and companies are shared across all of them.

**You decide:** the type. **The application does:** nothing else yet.

## 2. Add the candidate

Add a candidate in the Records area with their name and whatever contact details you hold. Records are shared, so a candidate added in one initiative is available in every other.

**You decide:** what is worth recording. **The application does:** validate the fields and refuse a malformed email or website rather than storing it.

## 3. Attach a document

Drop a resume onto the candidate. The document is stored exactly as you gave it — the bytes are never modified — along with where it came from.

**You decide:** what to attach. **The application does:** detect the type from the bytes, not the file name.

## 4. Extract it

Extraction converts the document to text. PDFs and Word files go through the bundled reader, one file per process, with no network access.

**You decide:** whether to retry a failure. **The application does:** record a short reason code if it cannot read the file, and never quote the document into a log.

## 5. Approve a profile

Classify the candidate. The model reads the extracted text and proposes typed, cited statements: skills, experience, location, work rights, pay expectations. Every statement must quote the source.

**You decide:** whether to approve it, edit a statement, or add one by hand. Nothing is used for searching or matching until you approve it.

**The application does:** refuse a proposal whose citations do not resolve, and mark the profile stale if the sources change afterwards.

## 6. State your criteria

Write what you are actually looking for. Criteria are your intent, kept separate from what the profile says.

**You decide:** the wording and the priority. **The application does:** refuse a criterion that names a protected attribute — age, race, sex and the rest — and say which one, so you can rewrite it lawfully.

## 7. Discover roles

Search for roles. Before anything is sent, you see the exact query — the bytes, not a description of it. Names, emails, phone numbers and addresses are removed, and the professional shape is kept.

**You decide:** whether to send it. **The application does:** record that a search happened, with no content in the record.

## 8. Build a shortlist

The shortlist ranks roles against the approved profile and your criteria. It searches two ways — by word and by meaning — and combines the results, so a role found by either can appear.

**You decide:** which roles are worth assessing. **The application does:** show why each role is on the list, and rank identically on repeated runs.

## 9. Read the assessment

An assessment compares one candidate and one role in both directions: does the candidate meet the role's requirements, and does the role meet the candidate's. Every verdict cites its evidence.

**You decide:** whether you agree. **The application does:** show the gaps as plainly as the matches.

## 10. Write and copy out a draft

Generate a pitch or an outreach message. It is built from approved evidence and shows which claim rests on which source.

**You decide:** the wording, and where it goes. **The application does:** let you edit and copy it — and nothing else. There is no send button anywhere, because the application has no way to send a message.
