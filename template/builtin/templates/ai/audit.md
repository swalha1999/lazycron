---
name: Audit
cron: every sunday at 3am
description: Weekly audit that files up to 3 verified issues (security, bugs, performance, debt) for the worker to pick up
tag: AU
tag_color: "#94e2d5"
model: opus
timeout: 90m
---
Audit this repository and file GitHub issues for the most valuable problems you can verify. Don't change any code.

1. Make sure the labels exist: `gh label create lazycron --color 5319e7 --force` (plus `security`, `bug`, `performance`, `tech-debt` if missing).
2. List what is already open: `gh issue list --state open --limit 100`. If 8 or more issues labeled `lazycron` are open, stop: the worker needs to catch up first.
3. Understand the project: README, layout, `git log --oneline -30`, then read the hot paths end to end.
4. Look for, in order:
   - Security: broken authz/authn, IDOR, injection, SSRF, XSS, secrets in code, data leaking into responses or logs.
   - Bugs: wrong control flow, unhandled errors, race conditions, resource leaks, logic that contradicts its comments or tests.
   - Performance and reliability: N+1 queries, unbounded work, missing indexes, slow pages.
   - Debt that hurts: real duplication, oversized modules, missing tests around critical paths.
   - For websites: broken links, accessibility, SEO/meta, Core Web Vitals.
5. Keep only findings you can pin to `file:line` with a concrete trigger and impact. No style nits, no generic advice, nothing already filed.
6. File at most 3, best first:
   `gh issue create --label lazycron --label <security|bug|performance|tech-debt> --title "<concise symptom>" --body "<problem with file:line refs / trigger / impact / proposed fix / acceptance criteria as a checklist>"`
   End each body with `Filed by the lazycron audit agent.`

If nothing meets the bar, file nothing and say so.
