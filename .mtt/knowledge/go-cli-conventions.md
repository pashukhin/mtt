---
title: Go/cobra conventions this codebase enforces
tags:
    - dx
priority: medium
created: "2026-07-23T07:59:10Z"
updated: "2026-07-23T07:59:10Z"
---
- cobra validates Args BEFORE RunE, on flag-stripped positionals: a context-sensitive command needs a
  context-sensitive PositionalArgs closure (it receives cmd with flags parsed), not a fixed arity.
- CLI output goes through fmt.Fprint(cmd.OutOrStdout(), ...) - cmd.Print* routes to stderr when no
  writer is set, breaking pipes and stdout asserts.
- Root sets SilenceErrors, so a unit test asserts the RETURNED error, not a SetErr buffer (the e2e
  harness differs: cli.Execute prints to real stderr).
- golangci unused fails on declared-but-unused package symbols: declare a symbol in the task that
  FIRST uses it; transient IDE unused diagnostics during multi-edit wiring are noise - make check is
  the gate.
- Exit-code taxonomy lives in Execute() int via errors.Is on core sentinels: wrap with %w everywhere
  (a %v silently degrades exit 4 to 1); a bulk best-effort aggregate must be a PLAIN fmt.Errorf -
  %w-wrapping one per-item error mis-maps the whole bulk.
- Zero-match --json emits [] not null: build with make([]T, 0, ...).
- Verb sugar rides root.RunE fallback with ArbitraryArgs (a real subcommand always wins the clash);
  route new forms to the OLD path (resolve edge-name -> target, call the existing runTransition) so
  gates/attribution/exit codes are inherited, not re-implemented.
