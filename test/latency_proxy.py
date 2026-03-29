#!/usr/bin/env python3
"""TCP latency proxy — adds configurable delay + jitter to all traffic.

Sits between clients and a TCP server (e.g. a WebSocket relay), forwarding
bytes in both directions with artificial delay. WebSocket-transparent: it
operates on raw TCP, so HTTP upgrade and WebSocket frames pass through as-is.

Usage:
    python3 test/latency_proxy.py --listen 127.0.0.1:3335 --target 127.0.0.1:3334 --latency 100 --jitter 30

    This proxies port 3335 → 3334 with 100ms base delay + 0-30ms random jitter
    on each forwarded chunk (both directions).
"""

import argparse
import random
import select
import socket
import sys
import threading
import time


def forward(src, dst, label, latency_s, jitter_s, stats):
    """Forward data from src to dst with delay."""
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            delay = latency_s + random.uniform(0, jitter_s)
            if delay > 0:
                time.sleep(delay)
            dst.sendall(data)
            stats[label] += len(data)
    except (ConnectionResetError, BrokenPipeError, OSError):
        pass
    finally:
        try:
            dst.shutdown(socket.SHUT_WR)
        except OSError:
            pass


def handle_conn(client_sock, target_addr, latency_s, jitter_s):
    """Handle one proxied connection."""
    try:
        server_sock = socket.create_connection(target_addr, timeout=5)
        server_sock.settimeout(None)  # back to blocking after connect
    except Exception as e:
        print(f"  proxy: connect to {target_addr} failed: {e}", file=sys.stderr)
        client_sock.close()
        return

    stats = {"c→s": 0, "s→c": 0}
    t1 = threading.Thread(target=forward, args=(client_sock, server_sock, "c→s", latency_s, jitter_s, stats), daemon=True)
    t2 = threading.Thread(target=forward, args=(server_sock, client_sock, "s→c", latency_s, jitter_s, stats), daemon=True)
    t1.start()
    t2.start()
    t1.join()
    t2.join()
    client_sock.close()
    server_sock.close()


def run_proxy(listen_addr, target_addr, latency_ms, jitter_ms):
    """Run the latency proxy."""
    latency_s = latency_ms / 1000.0
    jitter_s = jitter_ms / 1000.0

    lhost, lport = listen_addr.rsplit(":", 1)
    thost, tport = target_addr.rsplit(":", 1)
    target = (thost, int(tport))

    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind((lhost, int(lport)))
    sock.listen(64)
    print(f"latency proxy: {listen_addr} → {target_addr} (latency={latency_ms}ms jitter={jitter_ms}ms)")

    try:
        while True:
            client, addr = sock.accept()
            threading.Thread(target=handle_conn, args=(client, target, latency_s, jitter_s), daemon=True).start()
    except KeyboardInterrupt:
        pass
    finally:
        sock.close()


def main():
    p = argparse.ArgumentParser(description="TCP latency proxy")
    p.add_argument("--listen", default="127.0.0.1:3335", help="listen address (default: 127.0.0.1:3335)")
    p.add_argument("--target", default="127.0.0.1:3334", help="target address (default: 127.0.0.1:3334)")
    p.add_argument("--latency", type=int, default=100, help="base latency in ms (default: 100)")
    p.add_argument("--jitter", type=int, default=30, help="random jitter in ms (default: 30)")
    args = p.parse_args()
    run_proxy(args.listen, args.target, args.latency, args.jitter)


if __name__ == "__main__":
    main()
