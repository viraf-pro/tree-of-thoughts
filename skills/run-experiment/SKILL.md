---
name: run-experiment
description: Run an autonomous code experiment — modify code, execute, parse metrics, keep or discard. Use when the user wants to test a code change, benchmark an approach, or says "experiment", "try this code change", "benchmark", or "test this hypothesis".
---

# Run Experiment

Execute the two-phase experiment loop: prepare (patch + commit) then execute (run + evaluate).

## Prerequisites

The user needs an existing tree with an experiment config. If not configured:

1. Ask for: target file, run command, metric regex, metric direction (lower/higher), working directory
2. Call `configure_experiment` with these parameters
3. Confirm the config before proceeding

## Steps

1. **Check config.** Call `experiment_history` for the tree to see if experiments are configured and what the baseline is.

2. **Prepare the change.** The user provides a code change (or you generate one from the tree's current thought). Call `prepare_experiment` with:
   - `tree_id` — the active tree
   - `patch_content` — the full file content after modification
   - `commit_message` — descriptive message

   This writes the file and creates a git commit. Note the `previousHash` in the response — needed for rollback.

3. **Execute.** Call `execute_experiment` with:
   - `tree_id`
   - `node_id` — the thought node this experiment tests
   - `previous_hash` — from the prepare step

   The runner will:
   - Run the command with the configured timeout
   - Parse the metric from output via regex
   - Auto-evaluate the thought node (sure if improved, impossible if crashed)
   - Keep the commit if improved, rollback if not

4. **Report results.** Show:
   - Status: improved / regressed / crashed / timeout
   - Metric value vs baseline
   - Duration and memory usage
   - Whether the commit was kept or rolled back

5. **Next step.** If improved, suggest exploring deeper. If regressed, suggest trying a different approach from the frontier.

## Safety

- Always confirm the code change with the user before calling `prepare_experiment`
- The experiment runner handles git rollback automatically on failure
- Timeout is configurable (default 600 seconds)
