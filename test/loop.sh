#!/bin/bash
set -e
cd /home/mleku/src/orly.dev

# Compile all SW targets + app.
for d in sw sw-relay sw-marmot sm3sh; do
    echo "compile: next/$d"
    (cd next/$d && tinyjs -o ../../app/smesh3 .)
done

# Deploy to local test server (if deploy nsec is configured).
if [ -f ~/.config/sm3sh-deploy.env ]; then
    source ~/.config/sm3sh-deploy.env
    echo "deploy: sm3sh-deploy → http://sm3sh.test:8090"
    go run ./cmd/sm3sh-deploy --url http://sm3sh.test:8090 --nsec "$DEPLOY_NSEC"
    # Wait for SSE push + SW refresh.
    sleep 3
else
    echo "skip deploy: no ~/.config/sm3sh-deploy.env"
fi

# Clear log before test.
> /tmp/browser-debug.log

# Run Playwright.
echo "test: e2e.py"
python3 test/e2e.py
