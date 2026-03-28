#!/bin/bash
set -e
cd /home/mleku/src/orly.dev

# Compile all SW targets + app.
for d in sw sw-relay sw-marmot sm3sh; do
    echo "compile: next/$d"
    (cd next/$d && tinyjs -o ../../app/smesh3 .)
done

# Deploy to local test server (if deploy nsec is configured).
if [ -f ~/.config/smesh-deploy.env ]; then
    source ~/.config/smesh-deploy.env
    echo "deploy: smesh-deploy → http://smesh.test:8090"
    go run ./cmd/smesh-deploy --url http://smesh.test:8090 --nsec "$DEPLOY_NSEC"
    # Wait for SSE push + SW refresh.
    sleep 3
else
    echo "skip deploy: no ~/.config/smesh-deploy.env"
fi

# Clear log before test.
> /tmp/browser-debug.log

# Run Playwright.
echo "test: e2e.py"
python3 test/e2e.py
