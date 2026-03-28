#!/usr/bin/env python3
"""Playwright smoke test for smesh multi-SW bus connectivity."""

import sys
import time
from playwright.sync_api import sync_playwright

TEST_SK = "328615b6c92aa6527fc175a67670222daabc69fa2b84c2ded5f6907f78f2b0f8"
TEST_PK = "5dbeb1d7e84d0a4fb3fac47868438dc9135b35f25e27ae933e306e3584bf69a8"
BASE = "http://smesh.test:8090"
LOG_FILE = "/tmp/browser-debug.log"


def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=[
                f"--unsafely-treat-insecure-origin-as-secure="
                f"{BASE},"
                f"http://marmot.smesh.test:8090,"
                f"http://relay.smesh.test:8090",
            ],
        )
        ctx = browser.new_context()
        page = ctx.new_page()

        logs = []
        page.on("console", lambda m: logs.append(f"{m.type}: {m.text}"))

        # Inject key into localStorage before page JS runs.
        page.add_init_script(f"""
        localStorage.setItem('smesh-key', '{TEST_SK}');
        localStorage.setItem('smesh-pubkey', '{TEST_PK}');
        localStorage.setItem('smesh-mode', 'nsec');
        """)

        page.goto(BASE, wait_until="networkidle", timeout=30000)
        time.sleep(8)  # SWs register, bus connects, events flow

        # Read server-side log for SW bus connections.
        try:
            with open(LOG_FILE) as f:
                server_logs = f.read()
        except FileNotFoundError:
            server_logs = ""

        checks = {
            "page_loaded": page.title() != "",
            "app_render": page.evaluate(
                "document.querySelector('#app-root') !== null"
            ),
            "shell_bus": "bus connected" in server_logs
            and "[shell]" in server_logs,
            "relay_bus": "[relay]" in server_logs
            and "bus connected" in server_logs,
            "marmot_bus": "[marmot]" in server_logs
            and "bus connected" in server_logs,
            "no_errors": not any(
                "error" in l.lower()
                and "sw-diag" not in l
                and "favicon" not in l.lower()
                for l in logs
            ),
        }

        ok = True
        for name, passed in checks.items():
            status = "PASS" if passed else "FAIL"
            print(f"  {status}: {name}")
            if not passed:
                ok = False

        if not ok:
            print("\n--- PAGE CONSOLE LOGS ---")
            for l in logs:
                print(f"  {l}")
            if server_logs:
                print("\n--- SERVER LOGS (last 40 lines) ---")
                for l in server_logs.strip().split("\n")[-40:]:
                    print(f"  {l}")

        browser.close()
        sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
