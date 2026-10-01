---
title: Stop Running 15 Commands Over SSH: Meet Dockeretior, the Proactive Docker TUI & AutoDoctor #builtwithantigravity
published: true
description: A proactive terminal UI and intelligent diagnostic supervisor for Docker and Docker Compose. Root-cause analysis, visual dependency topologies, and 1-click healing.
tags: docker, devops, go, builtwithantigravity
canonical_url: https://github.com/mario-ezquerro/dockeretior
cover_image: https://raw.githubusercontent.com/mario-ezquerro/dockeretior/main/assets/banner.png
---

## 😫 The 2:00 AM SSH Nightmare

Every sysadmin, DevOps engineer, and backend developer knows this drill by heart:

Your monitoring ping rings. A production or staging server is sluggish. You SSH into the remote machine, open your shell, and start typing the ritual sequence of 15 commands:

```bash
docker ps -a
docker stats --no-stream
docker inspect --format='{{.State.ExitCode}}' <container>
docker logs --tail 50 <container>
docker system df
df -h
free -m
uptime
cat docker-compose.yml
```

You cross-reference logs, parse memory cgroups, try to decipher whether an **Exit Code 137** was an actual Linux OOM Killer invocation or a manual SIGKILL, and hunt down why your `/var/lib/docker` partition climbed from 55% to 89% in two weeks.

Web dashboards like Portainer exist, but they are heavy web apps that require exposing extra ports, configuring web servers, and opening remote ports. CLI tools like LazyDocker are great, but they are mostly passive viewers—they display tables, but don’t tell you:

> **"I detected 6 issues on this host: here are the 3 you must fix right now, why they happened, and how to fix them in one click."**

That is why I created **[Dockeretior](https://github.com/mario-ezquerro/dockeretior)**.

---

## ⚓ What is Dockeretior?

**Dockeretior** is an interactive, zero-overhead Terminal UI (TUI) and latent pseudo-terminal (PTY) supervisor written in Go. 

It runs directly in your terminal over SSH or local bash, featuring:

1. **Split-Screen Dashboard:** A live container list on the left alongside 4 real-time ASCII metric gauges on the right (CPU load & sparkline trends, RAM vs host cgroups, Network RX/TX throughput, Block I/O, PIDs, and uptime).
2. **🩺 AutoDoctor (Intelligent Root-Cause Diagnostics):** A health diagnostic engine that correlates Exit Codes, OOM Killer cgroups, binary entrypoint failures (126/127), restart loops, and security exposures into a **Server Health Score (0 - 100)** and a prioritized **Top 3 Remediation Plan**.
3. **🐙 Visual Docker Compose Topology:** Automatically parses multi-service Compose files and draws an ASCII dependency tree mapping services, `depends_on` directed graphs, volumes, and networks.
4. **💡 1-Click Storage Reclaim:** Identifies dangling images, orphaned volumes, and build cache (often 30+ GB of hidden reclaimable disk), letting you free it with a single keystroke (<kbd>c</kbd>).
5. **🚨 Alert Dispatcher & Drift Tracking:** Sends structured alerts to **Slack**, **Discord**, or **Telegram**, and tracks historical memory and disk creep across snapshots.
6. **⚡ Latent PTY Supervisor (Hot-Toggle):** Can run completely invisible in the background of your SSH shell, toggled into view in milliseconds using `Ctrl + \` or `Cmd + Option + Space`.

---

## 🩺 The Heart of Dockeretior: AutoDoctor

Instead of drowning the user in data, Dockeretior prioritizes actionable intelligence.

When you run `./bin/dockeretior --doctor` or press <kbd>a</kbd> inside the TUI, AutoDoctor inspects your host in sub-second time and outputs:

```text
================================================================================
🩺 AUTODOCTOR - SERVER HEALTH REPORT
================================================================================
Health Score : 58/100 🔴 Critical
Containers   : 3 running, 1 stopped (4 total)
Generated    : 2026-10-01 14:17:01
--------------------------------------------------------------------------------
SUBSYSTEMS STATUS:
  🟢 Docker Engine   : Docker v29.8.1, 16 CPUs, 31.3 GB RAM.
  🟢 Containers      : 3 active, 1 stopped.
  🔴 Storage         : 29.88 GB reclaimable in cache and unused images.
  🟡 Security        : 3 security alerts identified.
--------------------------------------------------------------------------------
TOP 3 PRIORITIZED ACTIONS:
  1. [🔴 CRITICAL] Significant Reclaimable Storage in Docker (29.88 GB) (Docker Storage)
     ↳ Root Cause: Accumulation of 14 dangling images, 11 orphaned volumes, and build cache.
     ↳ Solution  : Press 'c' inside AutoDoctor to reclaim disk space immediately.
     ↳ Command   : docker system prune -a --volumes

  2. [🔴 CRITICAL] Container 'buildkit_builder' running in privileged mode (--privileged)
     ↳ Root Cause: Privileged mode enabled, bypassing standard Docker container isolation.
     ↳ Solution  : Remove 'privileged: true' and grant only required Linux capabilities ('cap_add').

  3. [🟡 WARNING] PostgreSQL port exposed on 0.0.0.0 (5432:5432) on 'app-postgres'
     ↳ Root Cause: Public interface binding allows external connections from any IP.
     ↳ Solution  : Bind to '127.0.0.1:5432:5432' or rely on internal Docker bridge networks.
--------------------------------------------------------------------------------
💡 RECLAIMABLE DISK: 29.88 GB accumulated in unused images and build cache.
   To free space: launch dockeretior (press 'a' then 'c') or run docker system prune -a --volumes.
================================================================================
```

### Automatic Correlation of Crash Loops
When a container dies or starts restarting, AutoDoctor doesn't just show a red icon:
- **Exit Code 137:** Correlates whether the kernel invoked the OOM Killer and compares current RAM usage against the container's hard cgroup limit (e.g. `508 MB / 512 MB limit`).
- **Exit Code 126 / 127:** Flags missing binaries or entrypoint script permission issues (`chmod +x`).
- **Crash Loops:** Tracks restart frequency and isolates the exact stderr cause from the last seconds of the lifecycle.

---

## 🐙 Visual Docker Compose Dependency Tree

Have you ever opened a directory with a 400-line `docker-compose.yml` and wished you could just *see* the topology?

Pressing <kbd>c</kbd> in Dockeretior (or running `dockeretior --compose`) scans local Compose files, analyzes the Directed Acyclic Graph (DAG) of `depends_on`, and renders an ASCII architectural diagram:

```text
 📦 STACK: docker-compose.yml (4 services, 2 networks, 1 volume)

 ── Level 1: Ingress / Frontend / Entrypoint ──
  ┌──────────────────────────────┐  ┌──────────────────────────────┐
  │ 🚀 api                       │  │ 🌐 traefik                  │
  │ img: mycompany/api:v1.0      │  │ img: traefik:v2.10          │
  │ ports: 8080:8080             │  │ ports: 80:80,443:443        │
  └──────────────────────────────┘  └──────────────────────────────┘
            │
            ▼ (depends_on)
 ── Level 3: Databases / Cache / Storage ──
  ┌──────────────────────────────┐  ┌──────────────────────────────┐
  │ 🗄️  postgres                │  │ ⚡ redis                     │
  │ img: postgres:16-alpine      │  │ img: redis:7-alpine         │
  │ vols: 1 mounted              │  │ net: internal               │
  └──────────────────────────────┘  └──────────────────────────────┘

 ── Dependency & Link Matrix ────────────────────────
  • api              ──depends_on──▶ [postgres, redis]
```

---

## 🤖 #BuiltWithAntigravity: How It Came to Life

Building high-performance terminal software in Go is notoriously tricky:
- You have to handle **strict ANSI escape sequences** and raw terminal modes (`MakeRaw`).
- Terminal resize events (`SIGWINCH`) must be handled without tearing or buffer scrollbars.
- Go's build system has strict OS rules (e.g., naming a file `*_windows.go` excludes it from Linux/macOS builds).
- Concurrency between Docker event streams, PTY forwarders, and UI event loops must be rock-solid.

This entire project was engineered pair-programming with **Google Antigravity** using the **#builtwithantigravity** agentic workflow.

### What Google Antigravity enabled:
1. **Mathematical Layout Budgeting:** We ensured that every line rendered in the split-screen view calculates exact cell widths with `go-runewidth`, guaranteeing **zero terminal line-wrapping or viewport scroll jumps** across any window size (from compact 80x24 laptops to 4K ultrawide monitors).
2. **Deep Docker SDK Integration:** Implementing custom socket discovery (standard Unix socket, Docker Desktop macOS, Colima, OrbStack, and Linux rootless sockets).
3. **From Idea to Published Release:** In just a single continuous session, Antigravity helped:
   - Structure the comprehensive 11-phase architecture roadmap.
   - Implement the root-cause analysis rules engine.
   - Build the Compose DAG parser and ASCII box drawer.
   - Write comprehensive unit tests for every package (`go test -v ./...`).
   - Package CLI commands (`--doctor`, `--compose`, `--test-alert`).
   - Tag and publish release versions (`v1.1.0` through `v1.3.0`) directly to GitHub.

Pairing with an AI agent that understands deep systems-level programming, TTY intricacies, and Unix semantics was a game-changer for shipping this tool so quickly and robustly.

---

## 🚀 Quickstart: Try It in 30 Seconds

### Option 1: Build from Source (Go 1.22+)

```bash
# 1. Clone the repository
git clone https://github.com/mario-ezquerro/dockeretior.git
cd dockeretior

# 2. Build the binary
make build

# 3. Test instant diagnostics
./bin/dockeretior --doctor

# 4. Launch the full interactive TUI
./bin/dockeretior
```

### Option 2: Install via `go install`

```bash
go install github.com/mario-ezquerro/dockeretior/cmd/dockeretior@latest
```

---

## ⌨️ Essential Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| <kbd>F1</kbd> / <kbd>?</kbd> | **Help Modal:** Instant list of all keys and shortcuts |
| <kbd>F2</kbd> / <kbd>f</kbd> | **Filter:** Toggle between *Running only* and *All containers* |
| <kbd>F3</kbd> / <kbd>l</kbd> | **Live Logs:** Real-time container log streaming |
| <kbd>F4</kbd> / <kbd>e</kbd> | **Exec Shell:** Drop into an interactive container shell (`bash`/`sh`) |
| <kbd>F5</kbd> / <kbd>r</kbd> | **Restart:** Restart the selected container |
| <kbd>F6</kbd> / <kbd>s</kbd> | **Stop / Start:** Gracefully toggle container power state |
| <kbd>F7</kbd> / <kbd>p</kbd> | **Pause / Unpause:** Freeze or unfreeze container processes |
| <kbd>F8</kbd> / <kbd>x</kbd> | **Delete:** Delete container (with confirmation dialog or `f` to force) |
| <kbd>F9</kbd> / <kbd>i</kbd> | **Inspect:** View formatted container JSON configuration |
| <kbd>F10</kbd> / <kbd>q</kbd> | **Quit:** Cleanly exit and restore terminal state |
| <kbd>a</kbd> | **AutoDoctor:** Open server health score & root-cause report |
| <kbd>c</kbd> | **Compose / Prune:** Open Compose topology or assisted 1-click disk cleanup |
| <kbd>t</kbd> | **Topology Toggle:** Switch between visual DAG tree and directory explorer |

---

## 💡 What's Coming Next?

The roadmap for Dockeretior includes:
- **Ephemeral Test-Restore Backups:** Automated database dumps (PostgreSQL, MySQL) tested in ephemeral scratch containers to verify that your backup can *actually* be restored before disaster strikes.
- **Explain My Server:** Natural language host inspection based on real diagnostic data.
- **Docker Fleet Manager:** Managing multiple remote Docker hosts from a single central terminal.

Check out the code, star the project, and give it a spin:

⭐ **GitHub:** [https://github.com/mario-ezquerro/dockeretior](https://github.com/mario-ezquerro/dockeretior)

---

*What's the most frustrating Docker issue you run into when administering remote servers? Let me know in the comments below!*
