---
name: Worker
cron: every day at 5am
description: Takes the next unclaimed open issue, implements it, and opens a pull request
tag: WK
tag_color: "#f38ba8"
model: opus
timeout: 3h
---
Implement one GitHub issue end to end and open a pull request.

1. Make sure the labels exist: `gh label create lazycron:taken --color c5def5 --force`, `gh label create lazycron:blocked --color b60205 --force`, `gh label create lazycron:ready --color 0e8a16 --force`.
2. Pick the issue from ALL open issues (`gh issue list --state open --limit 100 --json number,title,labels,assignees`): `lazycron:ready` first, then oldest first. Skip any issue that is already claimed or not for an agent:
   - labeled `lazycron:taken` or `Taken` (another agent has it),
   - labeled `lazycron:blocked`, `future`, `wontfix`, `question`, `duplicate` or `invalid`,
   - assigned to anyone,
   - already referenced by an open pull request (`gh pr list --state open --search "<number>"`, then check the PR body for `#<number>`).
   If nothing is left, stop and say so.
3. Claim it right away so no other agent takes it: `gh issue edit <n> --add-label lazycron:taken`.
4. Read it with `gh issue view <n> --comments`, then explore the relevant code before changing anything.
5. `git switch -c lazycron/issue-<n>`, implement it so every acceptance criterion is met, add or adjust tests, and run the checks from the repository notes until they pass.
6. Commit referencing the issue, push the branch, and open the PR:
   `gh pr create --label lazycron --title "<what it does>" --body "Closes #<n>\n\n<what changed / how it was verified / what to look at>"`
   Then comment the PR link on the issue. Keep the `lazycron:taken` label on the issue until the PR is merged or closed.

If the issue is wrong, already fixed, too big for one PR, or needs a human decision (product choice, credentials, infrastructure), don't push anything. Comment on the issue explaining exactly what's needed, then run `gh issue edit <n> --remove-label lazycron:taken --add-label lazycron:blocked`.
