# Coding-template demo

A runnable, tested walkthrough of mtt's `coding` starter template
(`mtt init --template coding` → `feature` / `bugfix` / `refactor`), each type
driven end-to-end through its **gated Definition of Done**.

## Run it

```sh
make demo            # builds mtt, runs the walkthrough
# or, with mtt on PATH:
./demo/coding-flow.sh
```

Everything happens in a throwaway temp dir — nothing touches your project.

## What it shows

Each type deliberately hits a gate that **blocks** (exit 3), then passes:

- **feature** — `done` is blocked while the test is red; a feature needs it **green to finish**.
- **bugfix** — starting is blocked until a **failing repro test** exists (`! make test`); a bugfix needs it **red to start**.
- **refactor** — `done` is blocked by a public `pkg/` change (`git diff --exit-code -- pkg/`); the refactor must keep the public API stable.

## Tested

`coding_flow_test.go` builds mtt, runs the script, and asserts each type reached
`done` and each block fired — so `make check` keeps this demo honest.
