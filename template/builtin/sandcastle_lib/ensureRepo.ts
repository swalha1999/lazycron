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

/** Clones the repo on first call, then fetches + resets to defaultBranch. */
export function ensureRepo(opts: EnsureRepoOpts): string {
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
