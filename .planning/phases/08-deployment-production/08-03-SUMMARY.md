---
phase: 08-deployment-production
plan: 03
subsystem: infra
tags: [bash, curl, systemd, install-script, github-releases]

# Dependency graph
requires: []
provides:
  - install.sh for curl|bash deployment
  - systemd s-ui.service for开机自启
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [curl|bash install, systemd unit]

key-files:
  created: [install.sh]
  modified: []

key-decisions:
  - "GITHUB_REPO=HodgeWen/proxy-ui (actual repo from git remote)"
  - "INSTALL_DIR=/usr/local/s-ui per RESEARCH"
  - "Binary naming: s-ui-linux-amd64, s-ui-linux-arm64"

patterns-established:
  - "Pattern: install script with install/update/uninstall subcommands"
  - "Pattern: Port prompt with validation, config.json with panel schema"

# Metrics
duration: 12min
completed: 2026-02-12
---

# Phase 08 Plan 03: Bash Install Script Summary

**Bash install script (curl|bash) that downloads s-ui binary, creates systemd service, supports install/update/uninstall subcommands with interactive port prompt**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-02-12T08:42:39Z
- **Completed:** 2026-02-12
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- install.sh with core: download from GitHub releases, install to /usr/local/s-ui, systemd service
- Interactive port prompt (default 8080), validation, port-in-use check
- config.json creation with addr, session_secret, data_dir, singbox paths
- update: download latest, stop service, replace binary, start
- uninstall: stop, disable, optionally remove data dir

## Task Commits

Each task was committed atomically:

1. **Task 1: Install script core (download, install, systemd)** - `167b64c` (feat)
2. **Task 2: Interactive prompts and update/uninstall** - `99938ee` (feat)

## Files Created/Modified

- `install.sh` - Bash install script with install/update/uninstall subcommands

## Decisions Made

- GITHUB_REPO defaults to HodgeWen/proxy-ui (actual repo from git remote)
- Config schema: addr, session_secret (random), data_dir, singbox_config_path, singbox_binary_path
- Admin password: user sets at /setup after install (per CONTEXT)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## Self-Check

- [x] install.sh exists
- [x] Commits 167b64c and 99938ee exist

## Self-Check: PASSED

---
*Phase: 08-deployment-production*
*Completed: 2026-02-12*
