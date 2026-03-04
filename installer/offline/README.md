# Games Dashboard — Offline Bundle

An offline bundle packages all artifacts required to install Games Dashboard on
air-gapped or network-restricted hosts.  The installer's `--offline-bundle`
flag accepts the path to an extracted bundle directory.

---

## Bundle structure

```
games-dashboard-offline-<VERSION>-<ARCH>/
├── manifest.json          # Bundle metadata and per-file SHA-256 checksums
├── images/                # Docker images exported with `docker save`
│   ├── daemon.tar         # games-dashboard/daemon image
│   ├── ui.tar             # games-dashboard/ui image
│   ├── prometheus.tar     # prom/prometheus image
│   └── grafana.tar        # grafana/grafana image
├── helm/                  # Helm chart archives (.tgz)
│   ├── games-dashboard-<VERSION>.tgz
│   └── game-instance-<VERSION>.tgz
├── cli/                   # Pre-compiled gdash binaries
│   ├── gdash-linux-amd64
│   └── gdash-linux-arm64
├── steamcmd/              # SteamCMD archive (used by game adapters)
│   └── steamcmd.tar.gz
└── adapters/              # Game adapter YAML manifests
    ├── valheim.yaml
    ├── minecraft.yaml
    ├── satisfactory.yaml
    ├── palworld.yaml
    ├── eco.yaml
    ├── enshrouded.yaml
    └── riftbreaker.yaml
```

---

## Creating a bundle

Use the `build-offline-bundle.sh` script on a machine with internet access:

```bash
# Build a bundle for the current version targeting linux/amd64
VERSION=1.0.0 ARCH=amd64 \
  installer/scripts/build-offline-bundle.sh \
  --output /tmp/bundles

# The script produces:
#   /tmp/bundles/games-dashboard-offline-1.0.0-amd64.tar.gz
#   /tmp/bundles/games-dashboard-offline-1.0.0-amd64.tar.gz.sha256
```

---

## Verifying a bundle

Before installation, verify the bundle's integrity:

```bash
installer/scripts/verify-bundle.sh \
  /path/to/games-dashboard-offline-1.0.0-amd64.tar.gz
```

The script checks:
1. The outer archive SHA-256 against the `.sha256` sidecar file
2. Each artifact's SHA-256 against entries in `manifest.json`

Exit code `0` = verified; non-zero = tampered or incomplete bundle.

---

## Installing from a bundle

```bash
# Extract the bundle
tar -xzf games-dashboard-offline-1.0.0-amd64.tar.gz

# Run the installer pointing at the extracted directory
sudo installer/install.sh \
  --mode docker \
  --offline-bundle ./games-dashboard-offline-1.0.0-amd64 \
  --accept-licenses \
  --headless
```

The installer detects `--offline-bundle` and:
- Loads Docker images via `docker load` instead of pulling from a registry
- Installs Helm charts from the local `helm/` directory
- Installs SteamCMD from the bundled `steamcmd/steamcmd.tar.gz`
- Copies adapters from the bundled `adapters/` directory
- Installs the `gdash` CLI binary from `cli/`

---

## manifest.json schema

| Field            | Type   | Description                                      |
|------------------|--------|--------------------------------------------------|
| `version`        | string | Games Dashboard version (semver)                 |
| `arch`           | string | Target CPU architecture (`amd64`, `arm64`)       |
| `os`             | string | Target OS (`linux`)                              |
| `created_at`     | string | RFC3339 build timestamp                          |
| `builder`        | string | Hostname/CI job that built this bundle           |
| `artifacts`      | object | Map of artifact path → `{size, sha256}` entries |

See [bundle-manifest.json](bundle-manifest.json) for a concrete example.

---

## Security notes

- The `.sha256` sidecar and `manifest.json` entries guard against silent
  corruption or substitution during transit.
- For higher assurance, sign the `.sha256` file with GPG and distribute your
  public key out-of-band.  The `verify-bundle.sh` script will validate the
  GPG signature if a `.asc` file is present alongside the archive.
- Docker images are loaded with `docker load`, which executes no pull-time
  registry auth — only the images present in the bundle are used.
