# API documentation assets

These files are embedded into the Go binary by `internal/server/docs_handler.go`
(via `//go:embed all:docs`) and served at `/docs` and `/openapi.yaml`.

| File | Source |
| --- | --- |
| `openapi.yaml` | Hand-authored. Update this when adding a public REST endpoint that scripts/widgets are likely to use. Don't bother documenting purely UI-internal endpoints — see the curated-subset rationale at the top of `openapi.yaml`. |
| `index.html` | Hand-authored. Tiny Redoc loader. |
| `redoc.standalone.js` | **Vendored** from `https://cdn.jsdelivr.net/npm/redoc@2.1.5/bundles/redoc.standalone.js`. ~887 KB. |
| `redoc.standalone.js.LICENSE.txt` | Vendored alongside the bundle to satisfy MIT/BSD attribution requirements. |

## Why vendored, not built?

The CLAUDE.md "no committed build artifacts" rule targets *outputs of our own
build* (Go binaries, `web/dist/`). The Redoc bundle is a third-party release
artifact, not something our build produces. Vendoring it means:

- The Docker build is self-contained — no network fetch during image build.
- Documented version pinning lives in `git log` for this directory.
- Offline / air-gapped deployments work.

## Updating Redoc

```bash
REDOC_VERSION=2.1.5  # bump this
curl -fsSL -o redoc.standalone.js          "https://cdn.jsdelivr.net/npm/redoc@${REDOC_VERSION}/bundles/redoc.standalone.js"
curl -fsSL -o redoc.standalone.js.LICENSE.txt "https://cdn.jsdelivr.net/npm/redoc@${REDOC_VERSION}/bundles/redoc.standalone.js.LICENSE.txt"
```

Commit both files in the same commit; mention the version in the message.
