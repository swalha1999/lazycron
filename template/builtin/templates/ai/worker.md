---
name: Worker
cron: every 4 hours
description: Takes the next agent-filed or approved issue, implements it, and opens a pull request
tag: WK
tag_color: "#f38ba8"
model: opus
timeout: 3h
---
Implement one GitHub issue end to end and open a pull request.

1. Make sure the labels exist: `gh label create lazycron:taken --color c5def5 --force`, `gh label create lazycron:blocked --color b60205 --force`, `gh label create lazycron:ready --color 0e8a16 --force`.
2. Pick the issue: open issues labeled `lazycron:ready` first (the owner approved them), then `lazycron` (filed by agents), oldest first, skipping any labeled `lazycron:taken` or `lazycron:blocked` and any that an open PR already closes (`gh pr list --search "<number>"`). If none are left, stop and say so.
3. Claim it right away: `gh issue edit <n> --add-label lazycron:taken`.
4. Read it with `gh issue view <n> --comments`, then explore the relevant code before changing anything.
5. `git switch -c lazycron/issue-<n>`, implement it so every acceptance criterion is met, add or adjust tests, and run the project's checks until they pass.
6. Commit referencing the issue, push the branch, and open the PR:
   `gh pr create --label lazycron --title "<what it does>" --body "Closes #<n>\n\n<what changed / how it was verified / what to look at>"`
   Then comment the PR link on the issue.

If the issue is wrong, already fixed, too big for one PR, or needs a human decision (product choice, credentials, infrastructure), don't push anything. Comment on the issue explaining exactly what's needed, then run `gh issue edit <n> --remove-label lazycron:taken --add-label lazycron:blocked`.
