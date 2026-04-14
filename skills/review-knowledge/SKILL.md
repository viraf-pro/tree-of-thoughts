---
name: review-knowledge
description: Analyze the knowledge graph topology — find the most connected solutions, discover clusters, and identify surprising cross-domain connections. Use when the user says "show knowledge graph", "what do we know", "knowledge overview", "graph analysis", or wants to understand the shape of accumulated knowledge.
---

# Review Knowledge Graph

Analyze the structure and topology of the accumulated knowledge.

## Steps

1. **Graph analysis.** Call `knowledge_graph` to get:
   - **God nodes** — the most connected solutions (highest degree)
   - **Communities** — clusters of related solutions
   - **Bridge edges** — surprising connections between different domains

2. **Knowledge report.** Call `knowledge_report` for:
   - Top solutions by score
   - Tag coverage (what topics are well-covered vs sparse)
   - Recent events timeline
   - Suggested queries for exploration

3. **Retrieval stats.** Call `retrieval_stats` for:
   - Total solutions
   - How many have embeddings (vector search enabled)
   - Storage utilization

4. **Present the analysis.** Organize into:
   - **Map of knowledge** — what domains are covered, where the clusters are
   - **Most valuable solutions** — god nodes that many others reference
   - **Bridge insights** — unexpected connections between domains (these are often the most valuable)
   - **Gaps** — tags or areas with few solutions
   - **Health** — embedding coverage, orphan ratio

5. **Suggest actions.** Based on the analysis:
   - Areas to research next (sparse coverage)
   - Solutions to cross-reference (bridge opportunities)
   - Old solutions to compact (if many are aged)
   - Duplicate clusters to merge

## For Specific Topics

If the user asks about a specific area, call `retrieve_context` with that topic and then `get_solution_links` for each result to show the local graph around that topic.
