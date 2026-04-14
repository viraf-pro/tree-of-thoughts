---
name: export-vault
description: Export the knowledge base as an Obsidian vault with interlinked markdown files. Use when the user says "export", "obsidian", "browse knowledge", "visualize knowledge graph", or wants a visual overview of all stored knowledge.
---

# Export Knowledge Vault

Export the entire knowledge base as an Obsidian-compatible vault for visual browsing.

## Steps

1. **Check knowledge state.** Call `knowledge_report` to show what will be exported:
   - Number of solutions
   - Tag coverage
   - Link count

2. **Choose output directory.** Ask the user where to export, or use a sensible default (e.g., `~/tot-knowledge-vault`).

3. **Run the export.** Execute via the Bash tool:
   ```
   tot-mcp export --obsidian <output_dir>
   ```
   
   This creates:
   - One markdown file per solution with YAML frontmatter
   - Wiki-links (`[[solution-name]]`) between linked solutions
   - Tag-grouped index file
   - Graph-ready structure for Obsidian's graph view

4. **Report results.** Show:
   - Number of files created
   - Output directory path
   - How to open in Obsidian

5. **Suggest next steps:**
   - Open in Obsidian and enable the Graph View to see the knowledge topology
   - Use the tag index to navigate by domain
   - Solutions with many backlinks are the "god nodes" — start there
