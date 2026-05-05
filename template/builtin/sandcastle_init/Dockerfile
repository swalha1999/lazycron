FROM node:22-bookworm

# Install system dependencies
RUN apt-get update && apt-get install -y \
  git \
  curl \
  jq \
  && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
  | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && apt-get update && apt-get install -y gh \
  && rm -rf /var/lib/apt/lists/*

# Rename the base image's "node" user (UID 1000) to "agent".
RUN usermod -d /home/agent -m -l agent node

# Sandcastle's docker sandbox runs containers with `--user ${hostUid}:${hostGid}`,
# which on macOS is the host's UID (typically 501) — NOT 1000. The container
# therefore runs as a UID that doesn't own /home/agent, and writes to
# .gitconfig / .npm / etc. fail with "Permission denied". Make HOME world-
# writable so any host UID can use /home/agent as its home directory.
RUN chmod -R 0777 /home/agent

USER agent

# Install Claude Code CLI
RUN curl -fsSL https://claude.ai/install.sh | bash

# Add Claude to PATH
ENV PATH="/home/agent/.local/bin:$PATH"

# Re-apply 0777 so files added by the install scripts above remain writable
# from the runtime UID. Also pre-create /home/agent/.claude/ so the
# .credentials.json file mount from the host doesn't accidentally land on
# a missing parent (Docker would create an empty parent file otherwise).
USER root
RUN mkdir -p /home/agent/.claude/projects && chmod -R 0777 /home/agent
USER agent

WORKDIR /home/agent

# In worktree sandbox mode, Sandcastle bind-mounts the git worktree at ${SANDBOX_REPO_DIR}
# and overrides the working directory to ${SANDBOX_REPO_DIR} at container start.
ENTRYPOINT ["sleep", "infinity"]
