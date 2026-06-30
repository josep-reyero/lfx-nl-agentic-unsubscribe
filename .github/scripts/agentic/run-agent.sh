#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
#
# Run one pi agent over a brief and capture its verdict JSON. Mirrors the role of
# the old Codex run-agent: the agent's cwd is its own folder (its identity), and
# the verdict is written OUTSIDE the agent directory so a compromised run cannot
# tamper with it. The model is reached through GitHub Copilot (github-copilot/
# gpt-5.5); auth comes from $PI_CODING_AGENT_DIR/auth.json, minted beforehand by
# mint-copilot-token.sh.
#
# pi holds no GitHub token that can comment, approve, or merge. It only produces
# judgment as data; a separate deterministic step decides how that reaches
# GitHub. pi prints its final message (the verdict JSON) to stdout in -p mode,
# the same contract Codex's --output-last-message gave us.
#
# NOTE: tool restriction (read + diff only, no arbitrary bash) will come with the
# dedicated pi harness. Until then pi runs with its default tools.
#
# Usage: run-agent.sh <agent-dir-name> <output-json> <brief-file>
set -euo pipefail

AGENT="${1:?agent dir name required}"
OUT="${2:?output json path required}"
BRIEF="${3:?brief file required}"

: "${PI_CODING_AGENT_DIR:?PI_CODING_AGENT_DIR is required (run mint-copilot-token.sh first)}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
AGENT_DIR="$REPO_ROOT/agents/$AGENT"
[ -d "$AGENT_DIR" ] || { echo "ERROR: agent dir not found: $AGENT_DIR" >&2; exit 1; }
[ -f "$BRIEF" ] || { echo "ERROR: brief not found: $BRIEF" >&2; exit 1; }

MODEL="${PI_MODEL:-github-copilot/gpt-5.5}"
THINKING="${PI_THINKING:-xhigh}"

# --approve trusts the project-local config (the agent's .agents/skills) for this
# run. Without it pi blocks on its project-trust prompt at startup (the agent dir
# has .agents/skills), which has no TTY in CI and hangs forever.
# SECURITY: this trusts project-local files on the PR-head checkout, so a PR could
# add a malicious .pi/extension or .agents/skill. The dedicated pi harness must
# close this (load the brain from a pinned/trusted path, or strip PR-added
# .pi/.agents before running). Tracked for the harness work.
# GIT_TERMINAL_PROMPT=0 makes any git the agent runs fail fast instead of blocking
# on a credential prompt.
cd "$AGENT_DIR"
GIT_TERMINAL_PROMPT=0 pi -p --no-session --approve \
  --model "$MODEL" \
  --thinking "$THINKING" \
  "$(cat "$BRIEF")" \
  > "$OUT" 2> "$OUT.transcript.log"
