---
name: batch-ingest
description: Ingest multiple URLs or content items into the knowledge base in one pass. Use when the user shares multiple URLs, says "ingest these", "save all of these", "add these articles", or provides a list of resources to learn from.
---

# Batch Ingest

Process multiple knowledge items into the retrieval store.

## For Multiple URLs

1. **Collect the list.** The user provides URLs inline, in a file, or as a numbered list.

2. **Process sequentially.** For each URL:
   - Call `ingest_url` with the URL
   - Note: success/failure, title, size, assigned ID
   - If a URL fails (blocked, timeout, invalid), log it and continue

3. **Auto-tag.** After all items are ingested:
   - Look for common themes across the titles/content
   - Suggest a shared tag set
   - The user can accept or modify

4. **Cross-reference.** Call `lint_knowledge` to find auto-link opportunities between the newly ingested items and existing solutions.

5. **Report summary:**
   ```
   Ingested: 4/5 URLs
   Failed:   1 (https://... — 403 Forbidden)
   Tags:     [distributed-systems, consensus, raft]
   New links: 2 auto-detected
   ```

## For Mixed Content

If the user provides a mix of URLs and manual content:

1. URLs → `ingest_url`
2. Manual content → `store_solution` with problem/solution/tags
3. Process all items, then cross-reference

## For File-Based Lists

If the user points to a file with URLs (one per line):

1. Read the file with the Read tool
2. Extract URLs (one per line, skip blank lines and comments)
3. Process each with `ingest_url`

## Guidelines

- Process items sequentially (not in parallel) to avoid rate limiting
- If more than 10 items, confirm with the user before proceeding
- Always run `lint_knowledge` after batch ingestion to catch cross-reference opportunities
