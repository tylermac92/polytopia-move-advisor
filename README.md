# polytopia-move-advisor

## Pinned game build

All rules data (`rules/v1.yaml`) and scenario tests are pinned to one game build:

| Field | Value |
| --- | --- |
| Game build | `Android 2.17.3.16375` |
| Recording device | Samsung SM-S908U (Galaxy S22 Ultra) |
| OS | Android 16 |
| Auto-update | Disabled for Polytopia on the recording device |

**Finding the version in-game:** tap **About**, scroll to the bottom, then tap **Debug Info**.

**Before every recording session:** check the version on the Debug Info screen. If it doesn't match the game build above, stop recording scenarios. Scenarios from a different build are rejected by the test runner, and a new build needs a new rules file first (see "Pinning the game build" in the technical design doc).

The `game_build` value in `rules/v1.yaml` and the `build` field in every scenario file must match the game build above exactly.

## Development

Requires Go 1.24+ and [golangci-lint](https://golangci-lint.run) v2.

```sh
go build ./...
go vet ./...
go test ./...
golangci-lint run
```

CI (`.github/workflows/ci.yml`) runs all four on every push and pull request.

`internal/archtest` enforces the advisor's import boundary: `internal/search`, `internal/eval` and `internal/explain` must have no import path to `internal/sandbox`, `internal/view` or `internal/api`, and `internal/api` may reach the advisor only through `internal/search`.

Architecture decisions are recorded in [`docs/adr/`](docs/adr/).
