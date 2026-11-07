---
name: Improve
cron: every wednesday at 3am
description: Weekly focused code improvement opened as a pull request
tag: IM
tag_color: "#a6e3a1"
model: opus
timeout: 2h
---
Make one focused, high-value improvement to this codebase and open a pull request for it.

1. If an open PR labeled `lazycron:improve` already exists (`gh pr list --label lazycron:improve`), stop: one at a time.
2. Explore and pick ONE improvement with a clear, provable benefit: a robustness fix, removing real duplication, simplifying a convoluted function, tightening types, tests around an untested critical path, a measurable performance win, an accessibility fix. Skip anything already covered by an open issue or PR.
3. Avoid mass reformatting, codebase-wide renames, framework migrations, speculative abstractions and style-only churn.
4. `git switch -c lazycron/improve-$(date +%Y%m%d)`, implement it, and run the project's own checks (typecheck, lint, tests, build, whatever exists). They must pass.
5. Commit with a conventional-commit message, push the branch, then:
   `gh label create lazycron:improve --color 0e8a16 --force`
   `gh pr create --label lazycron --label lazycron:improve --title "<what it does>" --body "<what changed / why / how it was verified>"`

If nothing is clearly worth changing today, open nothing and say so.
