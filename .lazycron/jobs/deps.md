---
name: CVE + deps
cron: every day at 4am
description: Daily vulnerability check and dependency update opened as a pull request
tag: CV
tag_color: "#f9e2af"
model: sonnet
timeout: 90m
---
Keep this project's dependencies secure and current without breaking anything, and open (or refresh) one pull request.

1. Find the ecosystems and package managers from the lockfiles (pnpm-lock.yaml, package-lock.json, bun.lock, yarn.lock, Cargo.lock, go.sum, uv.lock, poetry.lock, requirements*.txt, ...). Use each lockfile's own manager; never switch managers or regenerate a lockfile from scratch.
2. Vulnerabilities first: run the native auditors (`pnpm audit`, `npm audit`, `bun audit`, `cargo audit`, `govulncheck ./...`, `pip-audit`, or `osv-scanner` if installed). Fix every advisory that has a fix; use overrides/resolutions only when a direct upgrade isn't possible.
3. Then bump patch and minor versions of direct dependencies. Take majors only when trivial and the checks pass; otherwise list them as skipped.
4. Install, then run typecheck, lint, tests and build. Revert any single upgrade that breaks something you can't fix cleanly.
5. If nothing changed, stop and say so.
6. `git switch -C lazycron/deps`, commit (`fix(security): ...` when CVEs were fixed, else `chore(deps): ...`), and `git push --force-with-lease -u origin lazycron/deps`.
7. If a PR from `lazycron/deps` is open, update its body with `gh pr edit`; otherwise:
   `gh label create lazycron:deps --color d93f0b --force`
   `gh pr create --label lazycron --label lazycron:deps --title "<summary>" --body "<advisories fixed (id, package, severity, from → to) / packages updated / skipped majors / verification>"`
