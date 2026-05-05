// ensureRepo — clone-once, fetch-every-run repo provisioning for sandcastle jobs.
//
// Used by .sandcastle/jobs/*.ts to make sure a fresh copy of the project
// repository is available before invoking sandcastle.run(). The base clone
// lives at ~/.lazycron-cache/repos/<name>/ and persists across runs. Every
// call fetches origin and resets the working tree to origin/<defaultBranch>,
// so the agent always operates on the latest main.
//
// Sandcastle's branchStrategy creates a per-run worktree off this base —
// concurrent agent runs do not conflict with each other.

import { existsSync } from "node:fs";
import { execSync } from "node:child_process";
import { homedir } from "node:os";
import * as path from "node:path";

export interface EnsureRepoOpts {
  /** Git URL (SSH or HTTPS). */
  url: string;
  /** Project name; used as the cache subdirectory. */
  name: string;
  /** Branch the working tree is reset to before each run. Default: "main". */
  defaultBranch?: string;
}

/**
 * requireEnv — fail fast with a friendly message if any of the named env
 * vars is missing or empty. Each agent template calls this at the top so
 * cron-fired runs surface "set X in .sandcastle/.env" instead of confusing
 * downstream errors like `git clone undefined`.
 */
export function requireEnv(...names: string[]): void {
  const missing = names.filter((n) => !process.env[n]);
  if (missing.length > 0) {
    console.error(
      `[lazycron] Missing required env var${missing.length > 1 ? "s" : ""}: ${missing.join(", ")}.\n` +
        `Set them in .sandcastle/.env (see .sandcastle/.env.example).`
    );
    process.exit(1);
  }
}

/** Clones the repo on first call, then fetches + resets to defaultBranch. */
export function ensureRepo(opts: EnsureRepoOpts): string {
  if (!opts.url) {
    throw new Error("ensureRepo: opts.url is required (set REPO_URL in .sandcastle/.env)");
  }

  const branch = opts.defaultBranch ?? "main";
  const cacheDir = path.join(homedir(), ".lazycron-cache", "repos", opts.name);

  if (!existsSync(cacheDir)) {
    execSync(`git clone ${opts.url} ${cacheDir}`, { stdio: "inherit" });
  }

  execSync(`git -C ${cacheDir} fetch origin --prune`, { stdio: "inherit" });
  execSync(`git -C ${cacheDir} checkout ${branch}`, { stdio: "inherit" });
  execSync(`git -C ${cacheDir} reset --hard origin/${branch}`, { stdio: "inherit" });

  return cacheDir;
}
