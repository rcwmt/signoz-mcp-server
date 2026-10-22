# Plan: Run pull-request CI on demand

Status: Done
Issue: https://github.com/rcwmt/sendpoly-temporal/issues/6113
PR: Published with this implementation

## Context

Automatic PR suites compete with release builds. Root40862 assigns this fork's CI gates; Root40984 requires ordinary PR checks to remain present and skipped, with existing runner/job semantics preserved. Issues are disabled in this fork, so Root40985 assigns the shared tracking issue. Root41019 supersedes the initial label-event-only gate with persistent label admission.

## Approach

Keep ordinary PR events and add labeled events plus manual dispatch. Heavy jobs run only on dispatch or a persistent `run-ci` label. Existing fork/Dependabot restrictions remain on reusable CI and e2e jobs; the privileged `safe-to-test` trigger cannot bypass demand. Include the event label (default `run-ci`) in workflow/ref concurrency so unrelated labels cannot cancel requested work; cancel only PR runs. Main-push, tag and release workflows are unchanged.

## Files to Modify

- `.github/workflows/{checks,ci,e2e,guardrails,mcp-protocol}.yaml` — event, job and concurrency gates.

## Verification

Installed actionlint passed on the five changed workflows. Static event-selection, concurrency and unchanged-job checks are recorded in the fleet handoff evidence. No local suites, builds, installs, dispatches or hosted executions were run, as ordered by Root40862. `make ci` and runtime CI execution remain unverified and Root-owned.

## Outcome

Implementation complete for source review. Adding `run-ci` admits CI; subsequent pushes rerun heavy jobs while the label remains. Removing it stops automatic heavy PR jobs. Existing upstream reusable jobs and runner declarations are retained, not migrated. No MCP tool, client contract, deployed service or agent-skills change.
