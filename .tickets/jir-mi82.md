---
id: jir-mi82
status: closed
deps: [jir-vlh4]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# Typed MutationResult for create, update, transition, comment, link, delete

Design 2.4. Replace Printer.Success(msg, keys...) with MutationResult{Key, URL, Action, Changes, Removed}. transition prints 'KEY: Backlog → Done'. --status matches the target status name first, then the transition name; an ambiguous match is a validation error listing the choices. Includes the step left out of the guarded update (design 1.4 step 4): after a description write, re-read and report the resulting state, with Removed naming nodes deleted under --force. Consider replacing IsProtectedToken with a Token field on ConversionIssue.

## Acceptance Criteria

Golden-file tests for the JSON shape of every mutating command; text output table-tested; ambiguous --status lists choices and writes nothing; update reports from/to for changed fields. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T09:19:21Z**

Merged to main in 2746aae after a clean second-opinion review and a green gate. Every mutating command prints a MutationResult (root mutation.go); 16 golden files under cmd/j4c/testdata/mutation pin the JSON. Every issue update reads first (one more GET); assign reads before and after. issue transitions prints ID, name and target status. ConversionIssue gained Token; IsProtectedToken is gone. For upload and delete: MutationResult may want an attachment ID field, as CommentID.
