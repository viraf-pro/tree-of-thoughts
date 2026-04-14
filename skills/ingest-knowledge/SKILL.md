---
name: ingest-knowledge
description: Ingest external knowledge into the store — URLs, articles, documentation, or manual solutions. Use when the user says "ingest", "save this", "remember this", "store this solution", "add to knowledge base", or shares a URL to learn from.
---

# Ingest Knowledge

Add external knowledge to the retrieval store for future use.

## For URLs

1. Call `ingest_url` with the URL and relevant tags
2. The server fetches the page, strips HTML, extracts the title, and stores as a solution
3. Content is automatically truncated to ~4000 chars and tagged "ingested"
4. Report: title, size fetched, assigned ID

## For Manual Solutions

When the user has a solution to store (not from a URL):

1. Ask for or infer:
   - **Problem statement** — what problem does this solve?
   - **Solution** — the answer or approach
   - **Tags** — categories for retrieval
2. If there's an active tree, use its ID. Otherwise pass empty tree_id.
3. Call `store_solution` with the details
4. The system auto-links to similar existing solutions

## For Batch Ingestion

If the user provides multiple URLs or solutions:

1. Process each one sequentially
2. After all are ingested, call `lint_knowledge` to check for new cross-reference opportunities
3. Report summary: how many ingested, any auto-links created

## After Ingestion

- Suggest relevant tags if the user didn't provide them
- Mention that `retrieve_context` can now find this knowledge
- If the ingested content relates to an active tree, suggest linking with `link_trees`
