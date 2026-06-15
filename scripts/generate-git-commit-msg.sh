#!/usr/bin/env bash
set -euo pipefail

LLM="claude" # default

# -----------------------------
# Parse arguments
# -----------------------------
for arg in "$@"; do
  case "$arg" in
    --claude)
      LLM="claude"
      ;;
    --gemini)
      LLM="gemini"
      ;;
    --codex)
      LLM="codex"
      ;;
    *)
      echo "Unknown option: $arg" >&2
      echo "Usage: $0 [--claude|--gemini|--codex]" >&2
      exit 1
      ;;
  esac
done

# -----------------------------
# Validate LLM CLI availability
# -----------------------------
if ! command -v "$LLM" >/dev/null 2>&1; then
  echo "Error: '$LLM' CLI not found in PATH." >&2
  exit 1
fi

# -----------------------------
# Ensure we're in a git repo
# -----------------------------
git rev-parse --git-dir >/dev/null 2>&1 || {
  echo "Not a git repository" >&2
  exit 1
}

# -----------------------------
# Ensure there are staged changes
# -----------------------------
if git diff --cached --quiet; then
  echo "No staged changes" >&2
  exit 1
fi

# -----------------------------
# Show staged files
# -----------------------------
echo ""
echo -e "\033[0;36m=== Staged Files ===\033[0m"
echo ""
git --no-pager diff --cached --name-status
echo ""

# -----------------------------
# Capture staged diff
# -----------------------------
DIFF="$(git --no-pager diff --cached)"

echo -e "\033[0;33mGenerating commit message using $LLM...\033[0m"

PROMPT=$(cat <<EOF
You are generating a git commit message that MUST follow
the Conventional Commits v1.0.0 specification.

Specification:
- Format: <type>[optional scope]: <description>
- Allowed types: feat, fix, docs, style, refactor, perf, test, chore
- Description:
  - imperative mood
  - lowercase
  - no trailing period
- Second line MUST be blank
- Body is optional, explains what and why (not how)
- Wrap body lines at ~72 characters
- No markdown
- No explanations
- No headers
- No analysis
- Output ONLY the commit message
- Do not specify a co-author
- Make sure the type matches the commit. i.e Do NOT mark a fix as a feat.

Input (staged diff):
$DIFF
EOF
)

# -----------------------------
# Call selected LLM
# -----------------------------
if [[ "$LLM" == "claude" ]]; then
  MSG="$(echo "$PROMPT" | claude --print)"
elif [[ "$LLM" == "gemini" ]]; then
  MSG="$(echo "$PROMPT" | gemini)"
elif [[ "$LLM" == "codex" ]]; then
  _tmpfile="$(mktemp)"
  codex exec --ephemeral -o "$_tmpfile" "$PROMPT" >/dev/null
  MSG="$(cat "$_tmpfile")"
  rm -f "$_tmpfile"
fi

# -----------------------------
# Confirm commit
# -----------------------------
echo ""
echo "$MSG"
echo ""
printf "Commit with this message? [y/N] "
read -r answer

if [[ "$answer" =~ ^[Yy]$ ]]; then
  git commit -S -m "$MSG"
else
  echo "Aborted." >&2
  exit 1
fi
