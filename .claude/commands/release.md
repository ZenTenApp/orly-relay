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

10. **Push to origin** with tags:
    ```
    git push origin main --tags
    ```

11. **Deploy monolithic binary to relay.orly.dev** (ARM64):
    Build and deploy the unified `orly` binary - a fully self-contained relay with all components embedded:
    - Embedded Badger/Neo4j database
    - Embedded ACL engine (follows, managed, curating modes)
    - Embedded NIP-77 negentropy sync
    - All subcommands (db, acl, relay) for split-mode if needed

    Build on remote (faster than uploading cross-compiled binary due to slow local bandwidth):
    ```bash
    ssh relay.orly.dev 'cd ~/src/next.orly.dev && git pull origin main && GOPATH=$HOME CGO_ENABLED=0 ~/go/bin/go build -o ~/.local/bin/orly ./cmd/orly'
    ssh root@relay.orly.dev '/usr/sbin/setcap cap_net_bind_service=+ep /home/mleku/.local/bin/orly && systemctl restart orly'
    ssh relay.orly.dev '~/.local/bin/orly version'
    ```

    Note: setcap must be re-applied after each binary rebuild to allow binding to ports 80/443.
    This is the recommended binary - just run `orly` for a complete monolithic relay,
    or use subcommands (`orly db`, `orly acl`) for split-mode architecture.

12. **Report completion** with the new version and commit hash

## Important:
- Always verify the build compiles before committing: `CGO_ENABLED=0 go build -o /dev/null ./...`
- If build fails, fix issues before proceeding
