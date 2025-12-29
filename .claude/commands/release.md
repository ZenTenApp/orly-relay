# Release Command

Review all changes in the repository and create a release with proper commit message, version tag, and push to remotes.

## Argument: $ARGUMENTS

The argument should be one of:
- `patch` - Bump the patch version (e.g., v0.35.3 -> v0.35.4)
- `minor` - Bump the minor version and reset patch to 0 (e.g., v0.35.3 -> v0.36.0)

If no argument provided, default to `patch`.

## Steps to perform:

1. **Read the current version** from `pkg/version/version`

2. **Calculate the new version** based on the argument:
   - Parse the current version (format: vMAJOR.MINOR.PATCH)
   - If `patch`: increment PATCH by 1
   - If `minor`: increment MINOR by 1, set PATCH to 0

3. **Update the version file** (`pkg/version/version`) with the new version

4. **Rebuild the embedded web UI** by running:
   ```
   ./scripts/update-embedded-web.sh
   ```
   This ensures the latest web UI changes are included in the release.

5. **Review changes** using `git status` and `git diff --stat HEAD`

6. **Compose a commit message** following this format:
   - First line: 72 chars max, imperative mood summary
   - Blank line
   - Bullet points describing each significant change
   - "Files modified:" section listing affected files
   - Footer with Claude Code attribution

7. **Stage all changes** with `git add -A`

8. **Create the commit** with the composed message

9. **Create a git tag** with the new version (e.g., `v0.36.0`)

10. **Push to remotes** (origin, gitea, and git.mleku.dev) with tags:
    ```
    git push origin main --tags
    git push gitea main --tags
    GIT_SSH_COMMAND="ssh -i ~/.ssh/gitmlekudev" git push ssh://mleku@git.mleku.dev:2222/mleku/next.orly.dev.git main --tags
    ```

11. **Deploy to VPS** by running:
    ```
    ssh relay.orly.dev 'cd ~/src/next.orly.dev && git stash && git pull origin main && export PATH=$PATH:~/go/bin && CGO_ENABLED=0 go build -o ~/.local/bin/next.orly.dev && sudo /usr/sbin/setcap cap_net_bind_service=+ep ~/.local/bin/next.orly.dev && sudo systemctl restart orly && ~/.local/bin/next.orly.dev version'
    ```

12. **Report completion** with the new version and commit hash

## Important:
- Do NOT push to github remote (only origin and gitea)
- Always verify the build compiles before committing: `CGO_ENABLED=0 go build -o /dev/null ./...`
- If build fails, fix issues before proceeding
