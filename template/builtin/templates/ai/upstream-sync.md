---
name: Upstream sync
cron: every day at 5am
description: For forks, merges the upstream repository into a pull request, resolving conflicts and keeping fork changes
tag: UP
tag_color: "#89b4fa"
model: opus
timeout: 90m
---
If this repository is a fork, bring in upstream's changes through one pull request.

1. Find upstream. If `.lazycron/context.md` or the repo notes name an upstream, use that. Otherwise use the GitHub parent: `gh repo view --json parent,defaultBranchRef`. If there is no upstream, stop: this isn't a fork.
2. `git fetch https://github.com/<upstream>.git <branch>` and count what's new: `git rev-list --count HEAD..FETCH_HEAD`. If 0, stop and say it's up to date.
3. `git switch -C lazycron/sync-upstream`, then `git merge --no-edit FETCH_HEAD`.
4. For each conflict, understand both sides: upstream's change, and our fork's intentional customization (check `git log` on our side for why it exists). Resolve so upstream improvements land and our customizations survive. Never take one side wholesale unless that's clearly right.
5. Run the project's build/checks if practical, and finish the merge commit.
6. `git push --force-with-lease -u origin lazycron/sync-upstream`. If a PR from that branch is open, update its body with `gh pr edit`; otherwise:
   `gh label create lazycron:sync --color 1d76db --force`
   `gh pr create --label lazycron --label lazycron:sync --title "chore: sync upstream <upstream> (<n> commits)" --body "<upstream commits summary / each conflict and how it was resolved / verification>"`

If you can't resolve a conflict confidently, run `git merge --abort`, push nothing, and open an issue labeled `lazycron` explaining which files conflict and why.
