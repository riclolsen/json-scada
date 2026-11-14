# DOX: platform-linux — Generic Linux Build & Management Scripts

## Purpose

Cross-distribution Linux scripts for building, exporting, and managing JSON-SCADA processes. These scripts work across Ubuntu, RHEL, and other Linux distributions.

## Ownership

- platform-linux owns generic Linux management scripts
- Distribution-specific installers (platform-ubuntu-*, platform-rhel*) own their automation

## Local Contracts

- `build.sh` — build all Go/Node.js/.NET components for Linux; also runs `src/cim-ems/install.sh` (Python venv for the CIM-EMS driver, skipped on non-x86_64)
- `restart_processes.sh` — restart all JSON-SCADA processes
- `restart_protocols.sh` — restart protocol driver processes only
- `export_project.sh` — export project configuration
- `import_project.sh` — import project configuration

## Work Guidance

- Scripts should be POSIX-compatible (bash + standard tools)
- Assume `supervisorctl` for process management
- Detect distribution at runtime for package-appropriate behavior

## Verification

- Run `build.sh` on both Ubuntu and RHEL to verify cross-distro compatibility
- Test `restart_processes.sh` and `restart_protocols.sh` with a running deployment
