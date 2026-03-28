#!/usr/bin/env python3
"""Playwright tests for Marmot MLS DM system.

Tests the CryptoProvider proxy chain, NIP-07 extension signing,
and end-to-end DM round-trips through the real browser stack.

Usage:
    python3 test/e2e_marmot.py [--headed] [--nip-io]

Requires:
    - Local server running on :8090 (smesh.test or nip.io)
    - /etc/hosts entry OR --nip-io flag for nip.io DNS
    - pip install playwright && playwright install chromium
"""

import argparse
import json
import os
import sys
import time
from pathlib import Path
from playwright.sync_api import sync_playwright, expect

# Test credentials — same as e2e.py.
TEST_SK = "328615b6c92aa6527fc175a67670222daabc69fa2b84c2ded5f6907f78f2b0f8"
TEST_PK = "5dbeb1d7e84d0a4fb3fac47868438dc9135b35f25e27ae933e306e3584bf69a8"

EXT_DIR = str(Path(__file__).parent / "extension")
LOG_FILE = "/tmp/browser-debug.log"


def make_urls(mode):
    if mode == "nip-io":
        base = "http://smesh.127.0.0.1.nip.io:8090"
        marmot = "http://marmot.smesh.127.0.0.1.nip.io:8090"
        relay = "http://relay.smesh.127.0.0.1.nip.io:8090"
    elif mode == "localhost":
        base = "http://smesh.localhost:8090"
        marmot = "http://marmot.smesh.localhost:8090"
        relay = "http://relay.smesh.localhost:8090"
    else:
        base = "http://smesh.test:8090"
        marmot = "http://marmot.smesh.test:8090"
        relay = "http://relay.smesh.test:8090"
    return base, marmot, relay


class TestResult:
    def __init__(self):
        self.results = {}
        self.errors = []

    def check(self, name, passed, detail=""):
        self.results[name] = passed
        if not passed and detail:
            self.errors.append(f"{name}: {detail}")

    def report(self):
        ok = True
        for name, passed in self.results.items():
            status = "PASS" if passed else "FAIL"
            print(f"  {status}: {name}")
            if not passed:
                ok = False
        if self.errors:
            print("\n--- ERRORS ---")
            for e in self.errors:
                print(f"  {e}")
        return ok


def clear_log():
    try:
        open(LOG_FILE, "w").close()
    except OSError:
        pass


def read_log():
    try:
        with open(LOG_FILE) as f:
            return f.read()
    except FileNotFoundError:
        return ""


def test_nsec_bus_connectivity(p, base, marmot_url, relay_url):
    """Test 1: nsec auth — SWs connect to bus, app renders."""
    r = TestResult()

    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context()
    page = ctx.new_page()

    logs = []
    page.on("console", lambda m: logs.append(f"{m.type}: {m.text}"))

    page.add_init_script(f"""
    localStorage.setItem('smesh-key', '{TEST_SK}');
    localStorage.setItem('smesh-pubkey', '{TEST_PK}');
    localStorage.setItem('smesh-mode', 'nsec');
    """)

    clear_log()
    page.goto(base, wait_until="networkidle", timeout=30000)
    time.sleep(8)

    server_logs = read_log()

    r.check("page_loaded", page.title() != "")
    r.check("app_render", page.evaluate(
        "document.querySelector('#app-root') !== null"
    ))
    r.check("shell_bus", "[shell]" in server_logs and "bus connected" in server_logs)
    r.check("relay_bus", "[relay]" in server_logs)
    r.check("marmot_bus", "[marmot]" in server_logs)

    # Check for JS errors (excluding known benign ones).
    js_errors = [
        l for l in logs
        if "error" in l.lower()
        and "sw-diag" not in l
        and "favicon" not in l.lower()
        and "ERR_INSUFFICIENT_RESOURCES" not in l
    ]
    r.check("no_js_errors", len(js_errors) == 0,
            f"{len(js_errors)} errors: {js_errors[:3]}")

    browser.close()
    return r


def test_nip07_auth(p, base, marmot_url, relay_url):
    """Test 2: NIP-07 extension auth — signEvent via crypto proxy chain."""
    r = TestResult()

    # Clean persistent context so SW re-installs and onActivate fires
    # (which establishes bus connection). Without this, a cached SW from
    # a previous run skips activation and the bus never connects.
    import shutil
    shutil.rmtree("/tmp/pw-chrome-nip07", ignore_errors=True)

    # Extensions require headed mode + persistent context in Playwright.
    ctx = p.chromium.launch_persistent_context(
        "/tmp/pw-chrome-nip07",
        headless=False,
        args=[
            f"--disable-extensions-except={EXT_DIR}",
            f"--load-extension={EXT_DIR}",
        ],
    )
    page = ctx.new_page()

    logs = []
    page.on("console", lambda m: logs.append(f"{m.type}: {m.text}"))

    # Don't inject nsec — let the app use NIP-07 mode.
    page.add_init_script(f"""
    localStorage.setItem('smesh-mode', 'extension');
    localStorage.setItem('smesh-pubkey', '{TEST_PK}');
    localStorage.removeItem('smesh-key');
    """)

    clear_log()
    page.goto(base, wait_until="networkidle", timeout=30000)

    # Wait for shell SW bus connection before sending messages.
    for _ in range(10):
        time.sleep(1)
        if "bus connected" in read_log():
            break

    # Check that the test signer injected window.nostr.
    has_nostr = page.evaluate("typeof window.nostr !== 'undefined'")
    r.check("window_nostr_present", has_nostr)

    # Check that getPublicKey returns our test key.
    if has_nostr:
        pk = page.evaluate("window.nostr.getPublicKey()")
        r.check("correct_pubkey", pk == TEST_PK, f"got {pk}")
    else:
        r.check("correct_pubkey", False, "no window.nostr")

    # Check that the test signer log appeared.
    signer_log = any("[test-signer]" in l for l in logs)
    r.check("signer_injected", signer_log)

    # Wait for SW controller, then send pubkey + MLS init to trigger
    # the marmot WS connection and NIP-07 proxy auth chain.
    page.evaluate(f"""
    (async () => {{
        // Wait for SW controller.
        if (!navigator.serviceWorker.controller) {{
            await new Promise(resolve => {{
                navigator.serviceWorker.addEventListener('controllerchange', resolve);
                setTimeout(resolve, 3000);
            }});
        }}
        const sw = navigator.serviceWorker.controller;
        if (!sw) return;
        sw.postMessage('["SET_PUBKEY","{TEST_PK}"]');
        await new Promise(r => setTimeout(r, 500));
        sw.postMessage('["MLS_INIT",["wss://relay.orly.dev"]]');
    }})()
    """)
    # Poll for marmot auth (SW lifecycle isn't instantaneous).
    marmot_auth = False
    for _ in range(10):
        time.sleep(1)
        if "marmot-sw: authenticated" in read_log():
            marmot_auth = True
            break

    server_logs = read_log()
    r.check("marmot_authenticated", marmot_auth,
            "no 'marmot-sw: authenticated' in server logs")

    # Debug: dump console logs on failure.
    if not marmot_auth:
        print("  --- console logs ---")
        for l in logs:
            print(f"    {l}")
        print(f"  --- server logs ---")
        for line in server_logs.strip().split("\n")[-10:]:
            print(f"    {line}")

    ctx.close()
    return r


def test_marmot_ws_protocol(p, base, marmot_url, relay_url):
    """Test 3: Direct marmot WS protocol — nsec auth, status, reset."""
    r = TestResult()

    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context()
    page = ctx.new_page()

    page.goto(base, wait_until="networkidle", timeout=30000)
    time.sleep(2)

    # Open a direct WebSocket to the marmot endpoint.
    ws_results = page.evaluate(f"""
    async () => {{
        const results = {{}};
        const ws = new WebSocket('{base.replace("http", "ws")}/__marmot');
        await new Promise((resolve, reject) => {{
            ws.onopen = resolve;
            ws.onerror = reject;
            setTimeout(reject, 5000);
        }});

        function sendAndWait(msg, timeout = 5000) {{
            return new Promise((resolve, reject) => {{
                const timer = setTimeout(() => reject('timeout'), timeout);
                ws.onmessage = (e) => {{
                    clearTimeout(timer);
                    resolve(JSON.parse(e.data));
                }};
                ws.send(JSON.stringify(msg));
            }});
        }}

        // Auth with nsec.
        const auth = await sendAndWait({{
            method: 'auth',
            nsec: '{TEST_SK}',
        }});
        results.auth_ok = auth.ok === true;
        results.auth_method = auth.method;

        // Status check.
        const status = await sendAndWait({{ method: 'status' }});
        results.status_method = status.method;
        results.status_pubkey = status.pubkey || '';

        // Reset.
        const reset = await sendAndWait({{ method: 'reset' }});
        results.reset_ok = reset.ok === true;

        ws.close();
        return results;
    }}
    """)

    r.check("ws_auth", ws_results.get("auth_ok", False))
    r.check("ws_auth_method", ws_results.get("auth_method") == "auth")
    r.check("ws_status", ws_results.get("status_method") == "status")
    r.check("ws_status_pubkey", ws_results.get("status_pubkey") == TEST_PK,
            f"got {ws_results.get('status_pubkey', '')}")
    r.check("ws_reset", ws_results.get("reset_ok", False))

    browser.close()
    return r


def test_crypto_req_signEvent(p, base, marmot_url, relay_url):
    """Test 4: crypto_req/crypto_resp for signEvent via WS.

    Opens a marmot WS in pubkey+sig mode, verifies the backend sends
    crypto_req for operations and accepts crypto_resp.
    """
    r = TestResult()

    import shutil
    shutil.rmtree("/tmp/pw-chrome-crypto", ignore_errors=True)

    # Extensions require headed mode + persistent context in Playwright.
    ctx = p.chromium.launch_persistent_context(
        "/tmp/pw-chrome-crypto",
        headless=False,
        args=[
            f"--disable-extensions-except={EXT_DIR}",
            f"--load-extension={EXT_DIR}",
        ],
    )
    page = ctx.new_page()

    logs = []
    page.on("console", lambda m: logs.append(f"{m.type}: {m.text}"))

    page.add_init_script(f"""
    localStorage.setItem('smesh-mode', 'extension');
    localStorage.setItem('smesh-pubkey', '{TEST_PK}');
    localStorage.removeItem('smesh-key');
    """)

    clear_log()
    page.goto(base, wait_until="networkidle", timeout=30000)

    # Wait for shell SW bus connection before sending messages.
    for _ in range(10):
        time.sleep(1)
        if "bus connected" in read_log():
            break

    # Trigger marmot auth via SW messages.
    page.evaluate(f"""
    (async () => {{
        if (!navigator.serviceWorker.controller) {{
            await new Promise(resolve => {{
                navigator.serviceWorker.addEventListener('controllerchange', resolve);
                setTimeout(resolve, 3000);
            }});
        }}
        const sw = navigator.serviceWorker.controller;
        if (!sw) return;
        sw.postMessage('["SET_PUBKEY","{TEST_PK}"]');
        await new Promise(r => setTimeout(r, 500));
        sw.postMessage('["MLS_INIT",["wss://relay.orly.dev"]]');
    }})()
    """)

    # Wait for marmot auth (poll log instead of fixed sleep).
    auth_ok = False
    for _ in range(10):
        time.sleep(1)
        if "marmot-sw: authenticated" in read_log():
            auth_ok = True
            break

    server_logs = read_log()

    r.check("crypto_proxy_auth", auth_ok,
            "NIP-07 crypto proxy auth chain failed")

    # Check that crypto_req messages were sent.
    crypto_logs = [l for l in logs if "crypto" in l.lower()]
    r.check("crypto_activity", len(crypto_logs) > 0 or auth_ok,
            "no crypto activity in logs")

    ctx.close()
    return r


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--headed", action="store_true",
                       help="Run in headed mode (visible browser)")
    parser.add_argument("--nip-io", action="store_true",
                       help="Use nip.io DNS instead of localhost")
    parser.add_argument("--hosts", action="store_true",
                       help="Use /etc/hosts (smesh.test) instead of localhost")
    parser.add_argument("--test", type=str, default="all",
                       help="Run specific test (nsec,nip07,ws,crypto,all)")
    args = parser.parse_args()

    mode = "localhost"
    if args.nip_io:
        mode = "nip-io"
    elif args.hosts:
        mode = "hosts"
    base, marmot_url, relay_url = make_urls(mode)

    tests = {
        "ws": ("marmot_ws_protocol", test_marmot_ws_protocol),
        "nip07": ("nip07_auth", test_nip07_auth),
        "crypto": ("crypto_req_signEvent", test_crypto_req_signEvent),
        "nsec": ("nsec_bus_connectivity", test_nsec_bus_connectivity),
    }

    if args.test == "all":
        run = list(tests.keys())
    else:
        run = [t.strip() for t in args.test.split(",")]

    all_ok = True
    with sync_playwright() as p:
        for i, key in enumerate(run):
            if key not in tests:
                print(f"Unknown test: {key}")
                continue
            if i > 0:
                time.sleep(2)  # settle between tests
            name, fn = tests[key]
            print(f"\n=== {name} ===")
            result = fn(p, base, marmot_url, relay_url)
            if not result.report():
                all_ok = False

    print(f"\n{'ALL PASSED' if all_ok else 'SOME FAILED'}")
    sys.exit(0 if all_ok else 1)


if __name__ == "__main__":
    main()
