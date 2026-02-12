# Stop Testing

Stop the local testing relay and smesh dev server.

## Argument: $ARGUMENTS

- `--clean` - Also remove all test data from `/tmp/orly-localtest/`
- (no argument) - Stop processes but keep data for next run

## Steps

1. Run the stop testing script:
   ```bash
   ./scripts/stoptesting.sh $ARGUMENTS
   ```

2. Report the result to the user.

## What it does

- Kills the relay launcher process tree (relay + db + acl subprocesses)
- Kills the smesh dev server
- Cleans up PID files
- With `--clean`: removes `/tmp/orly-localtest/` entirely
