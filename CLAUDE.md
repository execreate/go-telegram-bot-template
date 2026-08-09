# CLAUDE.md

See `README.md` for setup, configuration, architecture, and migration commands. This file covers only what the README
doesn't.

## This is a template

The repo is boilerplate other people fork. Keep changes generic and reusable — no domain- or business-specific features
unless explicitly asked. Prefer extending an existing pattern over introducing a new one.

## Verify before reporting done

```shell
go build ./... && go vet ./... && gofmt -l .   # gofmt must print nothing
```

Write `_test.go` tests for non-trivial new logic and run `go test ./...`.

## Keep the README in sync

Update the relevant README section in the same change when adding or altering:

- a config key → Configuration table (also `config.dist.yaml`, a getter in `configuration/config.go`, and
  `requiredConfig` in `main.go` if required)
- a handler → handler-execution-order table (also register it in `main.go` at the right group)
- a dependency, migration, or locale file

## Handler groups

Registration order in `main.go` doesn't matter; the group number does. -2/-1 are context enrichment, 0 is the T&C gate,
2+ are commands and may assume the user has accepted T&C.