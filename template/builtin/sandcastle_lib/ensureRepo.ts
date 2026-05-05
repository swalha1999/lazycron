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

/**
 * agentSandboxConfig — DockerOptions to pass into `docker(...)` so Claude
 * Code inside the container can authenticate the same way it does on the
 * host. Covers all three Claude auth mechanisms:
 *
 *   1. ANTHROPIC_API_KEY env var (API key)
 *   2. CLAUDE_CODE_OAUTH_TOKEN env var (OAuth subscription token)
 *   3. ~/.claude/.credentials.json file (set by `claude /login` on host)
 *
 * Whichever the user has, gets propagated. The host's ~/.claude is bind-
 * mounted read-only so the agent inherits the host's login without being
 * able to alter it. GH_TOKEN is also passed through for `gh` calls inside
 * the sandbox.
 */
export function agentSandboxConfig(): {
  env: Record<string, string>;
  mounts: { hostPath: string; sandboxPath: string; readonly?: boolean }[];
} {
  const env: Record<string, string> = {};
  for (const key of [
    "ANTHROPIC_API_KEY",
    "CLAUDE_CODE_OAUTH_TOKEN",
    "GH_TOKEN",
  ]) {
    const v = process.env[key];
    if (v) env[key] = v;
  }

  // Mount only the credentials file (not the whole ~/.claude/) so:
  //   - Claude inside the sandbox reads the host's login.
  //   - The sandbox keeps its own writable /home/agent/.claude/projects/,
  //     where Claude writes session logs that sandcastle then `docker cp`s
  //     out. Mounting the whole dir read-only blocked that write; mounting
  //     it read-write would leak agent session logs into the user's host.
  const mounts: { hostPath: string; sandboxPath: string; readonly?: boolean }[] = [];
  const credsFile = path.join(homedir(), ".claude", ".credentials.json");
  if (existsSync(credsFile)) {
    mounts.push({
      hostPath: credsFile,
      sandboxPath: "/home/agent/.claude/.credentials.json",
      readonly: true,
    });
  }

  return { env, mounts };
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
