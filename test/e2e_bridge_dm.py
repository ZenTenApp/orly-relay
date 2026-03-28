#!/usr/bin/env python3
"""E2E test: signer extension MLS + bridge MLS DM round-trip.

Selenium + Firefox — installs smesh-signer extension, creates vault,
generates key, logs into smesh, and tests MLS init + sendDM to the bridge.
The bridge speaks MLS only (marmot protocol, kinds 443/444/445).

Requires:
    - Local relay: ORLY_LISTEN=127.0.0.1 ORLY_SMESH3_DIR=$PWD/app/smesh3 ORLY_SMESH3_PORT=8090 /tmp/orly-local
    - Local bridge: ORLY_BRIDGE_RELAY_URL=ws://127.0.0.1:3334 ORLY_BRIDGE_PUBLIC_RELAY_URL=ws://127.0.0.1:3334
        ORLY_BRIDGE_DATA_DIR=~/.config/orly-bridge-test ORLY_BRIDGE_DOMAIN=bridge.test
        ORLY_BRIDGE_SMTP_PORT=2525 ORLY_BRIDGE_SMTP_HOST=127.0.0.1
        ORLY_BRIDGE_SMTP_RELAY_HOST=127.0.0.1 ORLY_BRIDGE_SMTP_RELAY_PORT=2525 /tmp/orly-local bridge
    - pip install --user --break-system-packages selenium
    - geckodriver (pacman -S geckodriver)
    - Signer extension: cd next/signer && bun run build:firefox
    Bridge npub (locked): npub1rzpltex05pjwmm4zuxehmpcptz2n2jt4xvp6kqls5nf9u7zwuprsk35xpd

Usage:
    python3 test/e2e_bridge_dm.py [--headed]
"""

import argparse
import json
import os
import re
import sys
import time
import subprocess

# Bridge identity — locked nsec in persistent config dir.
# npub1rzpltex05pjwmm4zuxehmpcptz2n2jt4xvp6kqls5nf9u7zwuprsk35xpd
BRIDGE_HEX = "1883f5e4cfa064edeea2e1b37d870158953549753303ab03f0a4d25e784ee047"
BRIDGE_DATA_DIR = os.path.expanduser("~/.config/orly-bridge-test")
BASE_URL = "http://127.0.0.1:8090"
XPI_PATH = "/tmp/smesh-signer.xpi"
EXT_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "next", "signer", "dist", "firefox")
VAULT_PASSWORD = "testpass123"
EXT_ID = "smesh-signer@orly.dev"

passed = 0
failed = 0


def get_bridge_pubkey():
    """Read bridge npub from bridge.nsec file and derive pubkey."""
    nsec_path = os.path.join(BRIDGE_DATA_DIR, "bridge.nsec")
    if not os.path.exists(nsec_path):
        return None, None
    # We can't easily derive hex from nsec in Python without dependencies.
    # Instead, read the bridge log output or use a subprocess.
    # For now, we'll get it from the relay by querying kind 443 events.
    return None, None


def build_xpi():
    manifest = os.path.join(EXT_DIR, "manifest.json")
    if not os.path.exists(manifest):
        print(f"Extension not found at {EXT_DIR}")
        print("Build it: cd next/signer && bun run build:firefox")
        sys.exit(1)
    subprocess.run(["zip", "-r", XPI_PATH, "."], cwd=EXT_DIR, capture_output=True, check=True)


def get_ext_uuid(profile_path):
    prefs = os.path.join(profile_path, "prefs.js")
    for _ in range(10):
        if os.path.exists(prefs):
            with open(prefs) as f:
                for line in f:
                    if "webextensions.uuids" not in line:
                        continue
                    m = re.search(r'"(\{.+\})"', line)
                    if not m:
                        continue
                    raw = m.group(1).replace('\\"', '"')
                    uuids = json.loads(raw)
                    if EXT_ID in uuids:
                        return uuids[EXT_ID]
        time.sleep(1)
    return None


def approve_prompt(driver, main_handle, By):
    handles = driver.window_handles
    approved = 0
    for h in handles:
        if h == main_handle:
            continue
        try:
            driver.switch_to.window(h)
            try:
                btn = driver.find_element(By.ID, "approveAlwaysButton")
                btn.click()
                approved += 1
                print(f"  approved prompt (always)")
                time.sleep(0.5)
            except Exception:
                try:
                    btn = driver.find_element(By.ID, "approveAllButton")
                    btn.click()
                    approved += 1
                    print(f"  approved all queued")
                    time.sleep(0.5)
                except Exception:
                    pass
        except Exception:
            pass
    if approved:
        time.sleep(1)
    try:
        driver.switch_to.window(main_handle)
    except Exception:
        pass
    return approved


def check(name, condition, detail=""):
    global passed, failed
    if condition:
        passed += 1
        print(f"  PASS: {name}")
    else:
        failed += 1
        print(f"  FAIL: {name}" + (f" — {detail}" if detail else ""))


def step(msg):
    print(f"\n{'='*3} {msg} {'='*3}")


# ─────────────────────────────────────────────
# Browser extension + MLS bridge tests
# ─────────────────────────────────────────────

def run_browser_tests(args):
    build_xpi()

    from selenium import webdriver
    from selenium.webdriver.firefox.options import Options
    from selenium.webdriver.firefox.service import Service
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.webdriver.support import expected_conditions as EC

    opts = Options()
    if not args.headed:
        opts.add_argument("--headless")
    opts.set_preference("xpinstall.signatures.required", False)
    opts.set_preference("extensions.autoDisableScopes", 0)
    opts.set_preference("devtools.console.stdout.content", True)

    svc = Service(log_output="/dev/null")
    driver = webdriver.Firefox(options=opts, service=svc)
    driver.set_script_timeout(60)

    try:
        _browser_tests(driver, args, By, WebDriverWait, EC)
    finally:
        if args.headed and sys.stdin.isatty():
            input("\npress Enter to close browser...")
        driver.quit()


def _browser_tests(driver, args, By, WebDriverWait, EC):
    profile = driver.capabilities.get("moz:profile", "")

    step("Install extension")
    driver.install_addon(XPI_PATH, temporary=True)
    time.sleep(2)
    uuid = get_ext_uuid(profile)
    check("extension UUID extracted", uuid is not None)
    if not uuid:
        return
    ext = f"moz-extension://{uuid}"
    print(f"  {ext}")

    step("Create vault")
    driver.get(f"{ext}/index.html")
    time.sleep(3)
    btn = WebDriverWait(driver, 10).until(
        EC.element_to_be_clickable((By.XPATH, "//button[.//span[contains(text(),'Create a new vault')]]"))
    )
    btn.click()
    time.sleep(1)
    pw = WebDriverWait(driver, 5).until(
        EC.presence_of_element_located((By.CSS_SELECTOR, "input[placeholder='vault password']"))
    )
    pw.send_keys(VAULT_PASSWORD)
    time.sleep(0.5)
    create = driver.find_element(By.XPATH, "//button[contains(text(),'Create vault')]")
    create.click()
    print("  Argon2id derivation...")
    WebDriverWait(driver, 15).until(lambda d: "home" in d.current_url)
    check("vault created", "home" in driver.current_url)

    step("Enable reckless mode")
    try:
        reckless = WebDriverWait(driver, 5).until(
            EC.presence_of_element_located((By.XPATH, "//input[@type='checkbox' and ancestor::*[contains(@class,'reckless')]]"))
        )
        if not reckless.is_selected():
            label = driver.find_element(By.XPATH, "//label[contains(@class,'reckless')]")
            label.click()
            time.sleep(0.5)
        check("reckless mode", True)
    except Exception as e:
        check("reckless mode", False, str(e))

    step("Add identity")
    add_btn = WebDriverWait(driver, 5).until(
        EC.element_to_be_clickable((By.CSS_SELECTOR, "button.add-btn"))
    )
    add_btn.click()
    time.sleep(2)
    nick = WebDriverWait(driver, 5).until(
        EC.presence_of_element_located((By.ID, "nickElement"))
    )
    nick.send_keys("e2e-test")
    gen = driver.find_element(By.XPATH, "//button[contains(text(),'Generate private key')]")
    gen.click()
    time.sleep(1)
    pk_input = driver.find_element(By.ID, "privkeyInputElement")
    nsec = pk_input.get_attribute("value")
    check("key generated", nsec and nsec.startswith("nsec1"), nsec[:20] if nsec else "empty")
    save = WebDriverWait(driver, 3).until(
        EC.element_to_be_clickable((By.XPATH, "//button[contains(text(),'Save')]"))
    )
    save.click()
    time.sleep(2)

    step("Select identity")
    identity_el = WebDriverWait(driver, 5).until(
        EC.element_to_be_clickable((By.CSS_SELECTOR, "div.identity"))
    )
    identity_el.click()
    time.sleep(1)
    is_selected = driver.execute_script(
        "return document.querySelector('div.identity.selected') !== null"
    )
    check("identity selected", is_selected)

    step("Login to smesh")
    driver.get(BASE_URL)
    time.sleep(5)
    has_nostr = driver.execute_script("return typeof window.nostr !== 'undefined'")
    check("window.nostr injected", has_nostr)

    main_handle = driver.current_window_handle
    # Click login
    for sel in ["//button[contains(text(),'login')]", "//button[contains(text(),'Login')]"]:
        try:
            btn = driver.find_element(By.XPATH, sel)
            if btn.is_displayed():
                btn.click()
                break
        except Exception:
            continue
    time.sleep(3)
    approve_prompt(driver, main_handle, By)
    time.sleep(3)

    # Get pubkey
    driver.execute_script("""
        window.__pk_result = null;
        window.nostr.getPublicKey()
            .then(k => { window.__pk_result = 'ok:' + k; })
            .catch(e => { window.__pk_result = 'err:' + e.message; });
    """)
    time.sleep(3)
    approve_prompt(driver, main_handle, By)
    time.sleep(3)
    pk = driver.execute_script("return window.__pk_result")
    if not pk:
        approve_prompt(driver, main_handle, By)
        time.sleep(3)
        pk = driver.execute_script("return window.__pk_result")
    check("getPublicKey", pk and pk.startswith("ok:"), pk)

    # Extract pubkey hex for later
    user_pubkey = pk.split(":", 1)[1] if pk and pk.startswith("ok:") else None

    # Wait for SW registration
    step("Wait for service workers")
    for _ in range(10):
        sw_ready = driver.execute_script("""
            return navigator.serviceWorker && navigator.serviceWorker.controller ? 'ready' : 'waiting';
        """)
        if sw_ready == 'ready':
            break
        time.sleep(2)
    print(f"  SW controller: {sw_ready}")
    check("service worker active", sw_ready == 'ready')

    # Check if SW iframes are present (relay SW, marmot SW)
    iframes = driver.execute_script("""
        var frames = document.querySelectorAll('iframe[id^="sw-iframe"]');
        return Array.from(frames).map(f => f.id + ' src=' + f.src);
    """)
    print(f"  SW iframes: {iframes}")

    step("MLS init")
    # First, tell the shell SW about relay URLs (it needs them for MLS_SUBSCRIBE routing)
    driver.execute_script("""
        if (navigator.serviceWorker && navigator.serviceWorker.controller) {
            navigator.serviceWorker.controller.postMessage('["MLS_INIT",["ws://127.0.0.1:3334"]]');
        }
    """)
    time.sleep(1)

    # Wait for window.nostr.mls to be available (content script injection can lag)
    for _ in range(10):
        has_mls = driver.execute_script("return !!(window.nostr && window.nostr.mls)")
        if has_mls:
            break
        time.sleep(1)

    # Retry init up to 3 times — extension port can be stale on first attempt
    result = None
    for attempt in range(3):
        driver.execute_script("""
            window.__mls_init = null;
            if (!window.nostr || !window.nostr.mls) {
                window.__mls_init = 'err:no mls'; return;
            }
            window.nostr.mls.init(['ws://127.0.0.1:3334'], 0)
                .then(r => { window.__mls_init = 'ok:' + r; })
                .catch(e => { window.__mls_init = 'err:' + e.message; });
        """)
        for _ in range(10):
            time.sleep(2)
            approve_prompt(driver, main_handle, By)
            result = driver.execute_script("return window.__mls_init")
            if result:
                break
        if result and result.startswith("ok:"):
            break
        if result and "Receiving end" in result:
            print(f"  mls.init attempt {attempt+1}: port stale, retrying...")
            time.sleep(2)
            continue
        break
    check("mls.init", result and result.startswith("ok:"), result)

    step("MLS publish key package")
    driver.execute_script("""
        window.__mls_kp = null;
        if (!window.nostr || !window.nostr.mls) {
            window.__mls_kp = 'err:no mls'; return;
        }
        window.nostr.mls.publishKP()
            .then(r => { window.__mls_kp = 'ok:' + r; })
            .catch(e => { window.__mls_kp = 'err:' + e.message; });
    """)
    for _ in range(10):
        time.sleep(2)
        approve_prompt(driver, main_handle, By)
        result = driver.execute_script("return window.__mls_kp")
        if result:
            break
    check("mls.publishKP", result and result.startswith("ok:"), result)

    step("MLS subscribe (start subscription loop)")
    driver.execute_script("""
        window.__mls_sub = null;
        if (!window.nostr || !window.nostr.mls) {
            window.__mls_sub = 'err:no mls'; return;
        }
        window.nostr.mls.subscribe()
            .then(r => { window.__mls_sub = 'ok:' + (r || 'started'); })
            .catch(e => { window.__mls_sub = 'err:' + e.message; });
    """)
    for _ in range(5):
        time.sleep(1)
        approve_prompt(driver, main_handle, By)
        result = driver.execute_script("return window.__mls_sub")
        if result:
            break
    check("mls.subscribe", result and result.startswith("ok:"), result)

    # Bridge pubkey is locked — same nsec persists across test runs.
    bridge_hex = BRIDGE_HEX
    print(f"  bridge pubkey: {bridge_hex}")

    # Set up MLS status listener before sending
    driver.execute_script("""
        window.__mls_statuses = [];
        window.__mls_dms = [];
        window.addEventListener('nostr-mls', (e) => {
            const d = e.detail;
            if (d && d.cmd === 'status') {
                window.__mls_statuses.push(d.msg);
                console.log('[e2e] MLS status: ' + d.msg);
            }
            if (d && d.cmd === 'dm') {
                window.__mls_dms.push(d);
                console.log('[e2e] MLS DM received: ' + JSON.stringify(d));
            }
        });
    """)

    # Also monitor console for MLS-related messages
    driver.execute_script("""
        window.__console_logs = [];
        const origLog = console.log;
        const origWarn = console.warn;
        const origErr = console.error;
        function capture(level, args) {
            var msg = Array.from(args).map(a => typeof a === 'string' ? a : JSON.stringify(a)).join(' ');
            if (msg.includes('mls') || msg.includes('MLS') || msg.includes('marmot') || msg.includes('subscribe') || msg.includes('[signer]') || msg.includes('relay') || msg.includes('publish')) {
                window.__console_logs.push(level + ': ' + msg);
            }
        }
        console.log = function() { capture('LOG', arguments); origLog.apply(console, arguments); };
        console.warn = function() { capture('WARN', arguments); origWarn.apply(console, arguments); };
        console.error = function() { capture('ERR', arguments); origErr.apply(console, arguments); };
    """)

    # ── Helper: send DM and wait for reply ──
    def send_and_wait(msg, label, timeout_rounds=20):
        """Send a DM to bridge and wait for a reply. Returns reply content or None."""
        escaped = msg.replace("\\", "\\\\").replace("'", "\\'").replace("\n", "\\n").replace("\r", "")
        driver.execute_script("window.__mls_dms = []; window.__mls_statuses = [];")
        driver.execute_script(f"""
            window.__mls_send = null;
            window.nostr.mls.sendDM('{bridge_hex}', '{escaped}')
                .then(r => {{ window.__mls_send = 'ok:' + r; }})
                .catch(e => {{ window.__mls_send = 'err:' + e.message; }});
        """)
        for _ in range(15):
            time.sleep(2)
            approve_prompt(driver, main_handle, By)
            result = driver.execute_script("return window.__mls_send")
            if result:
                break
        if not (result and result.startswith("ok:")):
            check(f"sendDM({label})", False, result)
            return None
        # Wait for reply DM
        for i in range(timeout_rounds):
            time.sleep(2)
            approve_prompt(driver, main_handle, By)
            dms = driver.execute_script("return window.__mls_dms")
            if dms:
                break
        if dms:
            content = dms[0].get("content", "")
            print(f"  ← reply: {content[:120]}")
            return content
        return None

    # ── 1. Initial status (before subscription) ──
    step("Send 'status' to bridge (before subscribe)")
    reply = send_and_wait("status", "status")
    check("status reply received", reply is not None, "no reply")
    if reply:
        check("status shows no subscription", "no active subscription" in reply.lower()
              or "no subscription" in reply.lower()
              or "expired" in reply.lower(), reply[:80])

    # ── 2. Subscribe ──
    step("Send 'subscribe' to bridge")
    reply = send_and_wait("subscribe", "subscribe")
    check("subscribe reply received", reply is not None, "no reply")
    if reply:
        check("subscription activated",
              "active" in reply.lower() or "payment received" in reply.lower()
              or "expires" in reply.lower(), reply[:120])

    # ── 3. Status after subscribe ──
    step("Send 'status' to bridge (after subscribe)")
    reply = send_and_wait("status", "status-after")
    check("status reply received (2)", reply is not None, "no reply")
    if reply:
        check("status shows active", "active" in reply.lower() or "expires" in reply.lower(), reply[:80])

    # ── 4. Send email to self via bridge ──
    step("Send email to self via bridge")
    # Build npub from user_pubkey for the email address
    npub_addr = driver.execute_script("""
        // Bech32-encode the pubkey for the email address.
        // Minimal bech32 encoder for npub.
        function bech32Encode(hrp, data) {
            const CHARSET = 'qpzry9x8gf2tvdw0s3jn54khce6mua7l';
            const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
            function polymod(values) {
                let chk = 1;
                for (const v of values) {
                    const b = chk >> 25;
                    chk = ((chk & 0x1ffffff) << 5) ^ v;
                    for (let i = 0; i < 5; i++) { if ((b >> i) & 1) chk ^= GEN[i]; }
                }
                return chk;
            }
            function hrpExpand(h) {
                const r = [];
                for (let i = 0; i < h.length; i++) r.push(h.charCodeAt(i) >> 5);
                r.push(0);
                for (let i = 0; i < h.length; i++) r.push(h.charCodeAt(i) & 31);
                return r;
            }
            function convertBits(data, fromBits, toBits, pad) {
                let acc = 0, bits = 0;
                const ret = [];
                const maxv = (1 << toBits) - 1;
                for (const v of data) {
                    acc = (acc << fromBits) | v;
                    bits += fromBits;
                    while (bits >= toBits) { bits -= toBits; ret.push((acc >> bits) & maxv); }
                }
                if (pad && bits > 0) ret.push((acc << (toBits - bits)) & maxv);
                return ret;
            }
            const bytes = [];
            for (let i = 0; i < data.length; i += 2) bytes.push(parseInt(data.substr(i, 2), 16));
            const words = convertBits(bytes, 8, 5, true);
            const chkData = hrpExpand(hrp).concat(words).concat([0,0,0,0,0,0]);
            const pm = polymod(chkData) ^ 1;
            const checksum = [];
            for (let i = 0; i < 6; i++) checksum.push((pm >> (5*(5-i))) & 31);
            return hrp + '1' + words.concat(checksum).map(v => CHARSET[v]).join('');
        }
        return bech32Encode('npub', window.__pk_result.split(':')[1]);
    """)
    email_addr = f"{npub_addr}@bridge.test"
    print(f"  email: {email_addr[:40]}...")

    email_body = f"To: {email_addr}\nSubject: E2E loopback\n\nHello from smesh E2E test"
    # Send email and collect ALL replies (confirmation + inbound loopback may arrive together)
    escaped = email_body.replace("\\", "\\\\").replace("'", "\\'").replace("\n", "\\n").replace("\r", "")
    driver.execute_script("window.__mls_dms = []; window.__mls_statuses = [];")
    driver.execute_script(f"""
        window.__mls_send = null;
        window.nostr.mls.sendDM('{bridge_hex}', '{escaped}')
            .then(r => {{ window.__mls_send = 'ok:' + r; }})
            .catch(e => {{ window.__mls_send = 'err:' + e.message; }});
    """)
    for _ in range(15):
        time.sleep(2)
        approve_prompt(driver, main_handle, By)
        result = driver.execute_script("return window.__mls_send")
        if result:
            break
    check("email DM accepted", result and result.startswith("ok:"), result)

    # ── 5. Wait for email confirmation + inbound loopback ──
    step("Wait for email round-trip (send confirmation + SMTP loopback)")
    # Wait for both the "Email sent" confirmation AND the inbound email DM
    got_confirmation = False
    got_inbound = False
    for i in range(25):
        time.sleep(2)
        approve_prompt(driver, main_handle, By)
        dms = driver.execute_script("return window.__mls_dms")
        for dm in (dms or []):
            c = dm.get("content", "").lower()
            if "email sent" in c or "sent to" in c:
                got_confirmation = True
                print(f"  ← confirmation: {dm.get('content', '')[:80]}")
            if "from:" in c and "subject:" in c:
                got_inbound = True
                print(f"  ← inbound email: {dm.get('content', '')[:120]}")
        if got_confirmation and got_inbound:
            break
        if got_inbound and i > 5:
            break  # inbound is enough — confirmation may have been missed

    check("email send confirmed or looped back", got_confirmation or got_inbound,
          f"confirm={got_confirmation} inbound={got_inbound}")

    # Check inbound email content
    inbound_content = ""
    dms = driver.execute_script("return window.__mls_dms") or []
    for dm in dms:
        c = dm.get("content", "")
        if "from:" in c.lower() and "subject:" in c.lower():
            inbound_content = c
            break
    if inbound_content:
        check("email contains subject", "e2e loopback" in inbound_content.lower(), inbound_content[:80])
        check("email contains body", "hello from smesh" in inbound_content.lower(), inbound_content[:80])
    else:
        check("inbound email received", False, "no inbound email DM found")

    # ── 6. Verify chat UI shows DMs ──
    step("Check chat UI — navigate to messages")
    # The sidebar has icon buttons: first child = feed, second child = messages.
    # Clicking the messages icon triggers switchPage("messaging") → initMessaging() → DM_LIST.
    driver.execute_script("""
        // Find the sidebar (44px wide column on the left) and click the 2nd icon button.
        var sidebar = document.querySelector('div[style*="width: 44px"], div[style*="width:44px"]');
        if (!sidebar) {
            // Fallback: find by structure — narrow flex column.
            var divs = document.querySelectorAll('div');
            for (var d of divs) {
                var s = d.style;
                if (s && s.width === '44px' && s.flexShrink === '0') { sidebar = d; break; }
            }
        }
        if (sidebar) {
            var icons = sidebar.children;
            if (icons.length >= 2) {
                icons[1].click();  // 2nd button = messages
                window.__sidebar_clicked = 'ok:' + icons.length;
            } else {
                window.__sidebar_clicked = 'err:only ' + icons.length + ' children';
            }
        } else {
            window.__sidebar_clicked = 'err:sidebar not found';
        }
    """)
    sidebar_result = driver.execute_script("return window.__sidebar_clicked")
    print(f"  sidebar click: {sidebar_result}")
    time.sleep(3)

    # Wait for DM_LIST response to populate conversations
    for _ in range(5):
        time.sleep(2)
        convos = driver.execute_script("""
            // The messaging page contains a list container. Conversation rows are
            // child divs with display:flex and cursor:pointer (clickable rows).
            // Find the visible messaging page, then look for flex row children.
            var pages = document.querySelectorAll('div[style*="padding: 16px"]');
            var msgPage = null;
            for (var p of pages) {
                if (p.style.display !== 'none' && p.style.position === 'relative') {
                    msgPage = p; break;
                }
            }
            if (!msgPage) return {found: false, reason: 'no msgPage visible'};
            // First child of msgPage is the list container.
            var listContainer = msgPage.firstElementChild;
            if (!listContainer) return {found: false, reason: 'no list container'};
            // Conversation rows: DIV children with cursor:pointer (not BUTTON = "new chat").
            var rows = [];
            for (var ch of listContainer.children) {
                if (ch.tagName === 'DIV' && ch.style && ch.style.cursor === 'pointer') {
                    rows.push(ch.textContent.substring(0, 80));
                }
            }
            return {found: rows.length > 0, count: rows.length, rows: rows.slice(0, 5)};
        """)
        if convos and convos.get("found"):
            break

    if convos and convos.get("found"):
        print(f"  conversations: {convos['count']} found")
        for r in convos.get("rows", []):
            print(f"    {r[:60]}")
        check("conversations visible in chat UI", True)
    else:
        print(f"  conversations: {convos}")
        check("conversations visible in chat UI", False, str(convos))

    # Open the bridge thread and verify messages
    thread_msgs = driver.execute_script(f"""
        // Click the first conversation row to open the thread.
        var pages = document.querySelectorAll('div[style*="padding: 16px"]');
        var msgPage = null;
        for (var p of pages) {{
            if (p.style.display !== 'none' && p.style.position === 'relative') {{
                msgPage = p; break;
            }}
        }}
        if (!msgPage) return null;
        var listContainer = msgPage.firstElementChild;
        if (!listContainer) return null;
        for (var ch of listContainer.children) {{
            if (ch.tagName === 'DIV' && ch.style && ch.style.cursor === 'pointer') {{
                ch.click(); break;
            }}
        }}
        return 'clicked';
    """)
    # Poll for message bubbles in the thread view (DM_HISTORY is async via bus)
    bubbles = []
    for attempt in range(8):
        time.sleep(2)
        bubbles = driver.execute_script("""
            var pages = document.querySelectorAll('div[style*="padding: 16px"]');
            var msgPage = null;
            for (var p of pages) {
                if (p.style.position === 'relative') { msgPage = p; break; }
            }
            if (!msgPage) return {msgs: [], debug: 'no msgPage'};
            var threadContainer = msgPage.children[1];
            if (!threadContainer) return {msgs: [], debug: 'no threadContainer'};
            if (threadContainer.style.display === 'none') return {msgs: [], debug: 'thread hidden'};
            // The message area is the flex:1 scrollable child.
            var msgArea = null;
            for (var ch of threadContainer.children) {
                if (ch.style && ch.style.flex === '1') { msgArea = ch; break; }
            }
            if (!msgArea) {
                // Fallback: the message area is the second child (index 1) of the thread container.
                // flex:1 may be normalized by the browser to flex-grow/flex-shrink/flex-basis.
                if (threadContainer.children.length >= 2) {
                    msgArea = threadContainer.children[1];
                }
                if (!msgArea) return {msgs: [], debug: 'no msgArea, children=' + threadContainer.children.length};
            }
            // Each message is a wrapper div containing a bubble div.
            var msgs = [];
            for (var wrapper of msgArea.children) {
                var bubble = wrapper.firstElementChild;
                if (bubble) {
                    var text = bubble.textContent.substring(0, 120);
                    var align = wrapper.style.justifyContent === 'flex-end' ? 'sent' : 'recv';
                    msgs.push({text: text, align: align});
                }
            }
            return {msgs: msgs, debug: 'msgArea children=' + msgArea.children.length};
        """)
        if bubbles and bubbles.get("msgs"):
            bubbles = bubbles["msgs"]
            break
        print(f"  thread poll {attempt}: {bubbles.get('debug', '?')}")
    else:
        bubbles = []

    if bubbles:
        print(f"  thread messages: {len(bubbles)} bubbles")
        for b in bubbles[:8]:
            print(f"    [{b['align']}] {b['text'][:80]}")
        check("messages rendered in thread", True)
        # Verify we see bridge replies
        has_bridge_reply = any("subscription" in b["text"].lower() or "email" in b["text"].lower()
                              or "active" in b["text"].lower() or "from:" in b["text"].lower()
                              for b in bubbles if b["align"] == "recv")
        check("bridge replies visible", has_bridge_reply)
    else:
        print(f"  thread messages: none rendered")
        check("messages rendered in thread", False, "no bubbles found")


# ─────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────

def main():
    global passed, failed

    parser = argparse.ArgumentParser()
    parser.add_argument("--headed", action="store_true")
    args = parser.parse_args()

    try:
        run_browser_tests(args)
    except Exception as e:
        print(f"\nTEST ERROR: {e}")
        import traceback; traceback.print_exc()
        failed += 1

    # Summary
    total = passed + failed
    print(f"\n{'='*40}")
    print(f"Results: {passed}/{total} passed, {failed} failed")
    print(f"{'='*40}")
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
