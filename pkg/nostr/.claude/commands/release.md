# Release Command

Create a new release with proper version tag and push to all configured remotes.

## Argument: $ARGUMENTS

The argument should be one of:
- `major` - Bump the major version (e.g., v1.0.9 -> v2.0.0)
- `minor` - Bump the minor version and reset patch to 0 (e.g., v1.0.9 -> v1.1.0)
- `patch` - Bump the patch version (e.g., v1.0.9 -> v1.0.10)

If no argument provided, default to `patch`.

## Steps to perform:

1. **Get the current version** from git tags:
   ```
   git tag --sort=-v:refname | grep '^v[0-9]' | head -1
   ```
   If no version tags exist, start from v0.0.0.

2. **Calculate the new version** based on the argument:
   - Parse the current version (format: vMAJOR.MINOR.PATCH)
   - If `major`: increment MAJOR by 1, set MINOR and PATCH to 0
   - If `minor`: increment MINOR by 1, set PATCH to 0
   - If `patch`: increment PATCH by 1

3. **Verify the build compiles**:
   ```
   go build ./...
   ```
   If build fails, stop and report the error. Do not proceed with release.

4. **Run tests**:
   ```
   go test ./...
   ```
   If tests fail, stop and report the error. Do not proceed with release.

5. **Review changes** using `git status` and `git log --oneline $(git describe --tags --abbrev=0 2>/dev/null || echo HEAD~10)..HEAD`

6. **If there are uncommitted changes**, compose a commit message following this format:
   - First line: 72 chars max, imperative mood summary
   - Blank line
   - Bullet points describing each significant change
   - Footer with Claude Code attribution

7. **Stage and commit changes** (if any):
   ```
   git add -A
   git commit -m "<message>"
   ```

8. **Create a git tag** with the new version:
   ```
   git tag -a <version> -m "Release <version>"
   ```

9. **Get all configured remotes** and push to each with tags:
   ```
   git remote | while read remote; do
     git push "$remote" main --tags
   done
   ```

10. **Report completion** with:
    - The new version number
    - The commit hash
    - List of remotes pushed to
    - Summary of changes since last release

## Important:
- Always verify build and tests pass before tagging
- Use annotated tags (`git tag -a`) for releases
- Push to ALL configured remotes (check with `git remote -v`)
- If any push fails, report which remote failed but continue with others
