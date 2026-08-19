---
id: documents-and-evidence
title: Documents and evidence
group: Working
summary: What happens to a file you attach, and what "cited" means here.
---

## Artifacts are immutable

A document you attach is stored byte for byte. It is never edited, re-saved, or normalised. If the same file is attached twice it is recognised as the same bytes.

Each artifact records where it came from: uploaded by you, pasted as text, or retrieved from a search.

## Extraction

Extraction turns a document into text. Plain text and Markdown are read directly. PDFs and Word files go through the bundled reader, which runs as a separate process with plugins and network access switched off, one file at a time, with limits on time, memory, and output size.

Temporary files live inside your data folder — the encrypted one — and are cleaned up after use and again at the next startup.

If extraction fails you get a short reason: the file could not be read, the reader was not found, it ran out of time, or it produced too much output. The reason is a code, never the reader's own words, because those can quote the document.

## Chunks and citations

Extracted text is split into sections. A section is the unit everything else points at: a profile statement cites the section it came from, and quotes wording that actually appears there.

When a statement claims something, you can open the section and read the sentence. If a quote does not appear in the section it claims, the whole profile is refused rather than stored with a citation that does not check out.

## Searching your evidence

Search finds sections by word. Semantic search finds them by meaning, which needs the embedding model and an indexed workspace. Both show which document and which heading a result came from.
