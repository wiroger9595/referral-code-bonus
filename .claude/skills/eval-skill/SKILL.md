---
name: eval-skill
description: This skill should be used when the user wants to evaluate, test, benchmark, or regression-check an existing agent skill — measuring whether the skill triggers on the right queries, whether it actually improves agent behavior compared to a baseline, whether a discipline skill holds up under pressure, or whether a skill still works after edits or a model update — or wants to execute a saved eval runbook from the repo's runbooks/ directory. Trigger phrases include "evaluate this skill", "test my skill", "benchmark skill performance", "run skill evals", "run the runbook", "did this skill regress", "is the new version better". Also invoked by create-skill as the quality gate inside its creation loop.
---

# Eval Skill

Evaluate an existing skill with evidence instead of vibes.

**Core principle: a skill's value is a delta, never an absolute.** Every measurement pairs a with-skill run against a baseline. A run that passes without a baseline comparison proves nothing; an eval the baseline already passes proves the skill is unnecessary or the eval is broken.

Quality standards being checked live in `references/quality-standards.md` (rule IDs cited throughout). How to design scenarios lives in `references/scenario-design.md`. Data contracts and the workspace layout live in `references/schemas.md`.

## CLI resolution (once per session)

There is no launcher script — resolving the CLI is your job, once, before the first command. From the eval-skill skill root:

```text
1. bin/ holds this platform's compiled binary?  ->  use that path
     macOS:    bin/eval-skill-darwin-arm64
     Linux:    bin/eval-skill-linux-x64
     Windows:  bin/eval-skill-windows-x64.exe
2. no binary for this platform:  the CLI is unavailable -- report that
     and stop; never improvise a substitute
```

Every `eval-skill …` command below means the command resolved here.

## The evaluation pyramid

Four tiers, cheap to expensive. Pick tiers by what changed; don't run expensive tiers to answer cheap questions.

| Tier | What | Cost | When required |
|------|------|------|---------------|
| 0 | Static lint (`eval-skill validate-skill`) | seconds | Always |
| 1 | Trigger testing (does it fire on the right queries?) | minutes | Always |
| 2 | Behavioral lift (paired with-skill vs baseline runs, graded) | subagent-heavy | Behavioral changes; shipping gate |
| 3 | Pressure testing (does discipline hold under stress?) | subagent-heavy | Discipline-enforcing skills |

Mechanical edits (typos, path fixes, formatting) need only Tier 0 plus a re-run of the existing suite. Behavioral changes need the tier that matches what changed: description → Tier 1; body content → Tier 2; rules with compliance costs → Tier 3 as well.

## Tier 0: Static lint

All `eval-skill …` commands in this document run from the eval-skill skill root. In the repo-mirror layout (`skills/`, `runbooks/`, `result/` at the repo root), running from the skill root has two path consequences:

- The skill root sits two directory levels below the repo root, so every literal `runbooks/<id>.yaml` and `result/<id>/...` path shown in this document needs a two-levels-up prefix when copy-pasted as written (e.g. `../../runbooks/<id>.yaml`, `--workspace ../../result/<id>/iteration-N`).
- A subcommand that resolves a path against the CLI's cwd rather than the repo root — `run-iteration`'s and `run-behavioral`'s `--repo-root` (default: cwd), `run-eval`'s `--skill-path` default (`target_skill` resolved against cwd) — needs `--repo-root=../..` or an explicit override, never its bare default, when invoked from here.

```bash
eval-skill validate-skill <path-to-skill-dir>
```

Checks structure, frontmatter, size budgets, leftover TODO scaffold markers, dead references, forbidden auxiliary files, and description heuristics — the lint-checkable subset of `references/quality-standards.md`. Zero FAILs is the bar; investigate WARNs rather than dismissing them.

## Tier 1: Trigger testing

The description is the skill's only triggering mechanism, so test it like one: with a query set, not by rereading it.

1. Build (or load from the target's runbook → `trigger_evals`) 16–20 realistic queries, half should-trigger and half **near-miss** should-not-trigger. Design rules: `references/scenario-design.md` § Trigger scenarios.
2. For each query, run ≥ 3 fresh-context subagent trials: give the subagent the query as its task, with the skill's name + description present in its available-skills context, and observe whether it consults the skill. Record trigger rate per query.
3. Score as precision/recall. Bar: should-trigger rate ≥ 2/3 per query, near-miss false-trigger rate ≤ 1/3.
4. When sibling skills exist, run their should-trigger queries too — stealing a sibling's traffic is a regression.

**Preferred (headless CLI):** `eval-skill run-eval` measures the real harness decision by injecting the candidate description into live headless sessions (`claude -p` or `codex exec --json`) — more truthful than subagent estimation. Use it whenever a CLI is available; fall back to subagent trials (step 2 above) only without one.

- **Platform flag.** The current agent selects its own platform explicitly on every script invocation: Codex passes `--cli=codex`; Claude Code passes `--cli=claude`. There is no auto-detection and no default. This rule binds every `--cli` flag in this document — commands below write `--cli=<claude|codex>` once instead of spelling both variants out.
- **Input mode.** `--runbook` consumes a runbook's `trigger_evals[]` directly; `--eval-set` takes a standalone eval file. The two are mutually exclusive.
- **`--skill-path` is a trap in runbook mode.** It defaults to the runbook's `target_skill` resolved against the current working directory — and since that directory is the skill root (Tier 0), the default resolves to `<skill-root>/<target_skill>`, which doesn't exist. Always state `--skill-path` explicitly.
- **Repetitions.** In runbook mode `--runs-per-query` defaults to the runbook's `runs_per_configuration` (its own standalone default is 3 otherwise) — one field dials repetitions for Tier 1 and Tier 2/3 alike. Pass `--runs-per-query` explicitly to diverge from it.

```bash
# Runbook mode (paths relative to the skill root — Tier 0)
eval-skill run-eval --cli=<claude|codex> --runbook ../../runbooks/<id>.yaml --skill-path ../../<target_skill> [--model <model-id>] [--effort <value>]

# Standalone eval-set mode
eval-skill run-eval --cli=<claude|codex> --eval-set <trigger-eval.json> --skill-path <skill> [--model <model-id>] [--effort <value>]
```

Trigger rates are CLI-scoped: a Claude-measured rate and a Codex-measured rate are two separate results, never pooled or compared against each other. Persist the concrete `cli` value in the result.

To optimize a weak description automatically, `eval-skill run-loop` wraps `run-eval` in an improve-and-re-evaluate loop with a train/test split (best description chosen by held-out score, to avoid overfitting), calling `eval-skill improve-description` for rewrites and `eval-skill generate-report` for the report. Invoke the loop with the same explicit `--cli` flag so measurement and rewriting stay in one CLI stratum. Curate the query set with the user first via `assets/eval_review.html` — bad eval queries produce bad descriptions.

## Tier 2: Behavioral lift

### Run

**Pick the carrier before anything else.** The runbook's `executor` decides how every Tier 2/3 run is carried out, and the two carriers are not interchangeable — full contracts in `references/execution-modes.md`:

- `cli` — each run hermetic in its own disposable sandbox (in-vitro), artifacts written mechanically. The only carrier whose numbers may close a gate.
- `subagent` — runs fan out in-harness; fast, and what a scaffolded runbook ships with. The baseline arm still runs through the hermetic CLI, because a subagent baseline inherits this session's skills and is therefore not skill-free. That asymmetry makes the lift an **upper bound**, `check-trace` unavailable, and blinding a discipline rather than a guarantee — all of which the report must state.

Tier 1 sits outside this choice: it measures the production harness's real discovery decision, so it runs the real CLI whatever the carrier — in a throwaway sandbox of its own, where the synthesized two-field candidate is the only project-level skill and the user-level layer still loads, so the description still competes against a realistic field. (It once injected into the live project root instead; a leftover from a killed run quietly diluted every later measurement in that repo.)

1. **Freeze the contract and the inputs before any run.** Write a one-page contract: target version, baseline definition, repetitions, tiers in scope, gates, and the concrete CLI (`codex` or `claude`) selected from the current agent. Create the workspace per `references/schemas.md` § Workspace layout. Then:
   - Freeze the target skill and every fixture with `eval-skill freeze-inputs write --workspace <workspace> <skill-dir> <fixture-files>...` before the first run (writes `input-hashes.sha256`, a shasum-compatible manifest). After runs, re-verify with `eval-skill freeze-inputs check --workspace <workspace>` and treat any `MISMATCH`/`MISSING` line as a broken iteration.
   - Know the manifest's scope: it hashes every file under `<skill-dir>` except `__pycache__`/`*.pyc` — no binary-awareness of its own. Binary provenance rides in `sandbox-base/` instead (`references/schemas.md` § Workspace layout).
   - When evaluating an improvement to an existing skill, snapshot the old version first (`cp -r <skill> <workspace>/skill-snapshot/`) — that snapshot is the baseline.
2. Freeze assertions into each eval's `eval_metadata.json` before spawning anything (design rules: `references/scenario-design.md` § Behavioral scenarios) — derived from the skill's claimed outcomes, never drafted or tightened after seeing outputs.
3. **Execute the iteration with one command:** `eval-skill run-iteration --cli=<claude|codex> --runbook ../../runbooks/<id>.yaml --repo-root=../.. --workspace ../../result/<id>/iteration-N [--model ...] [--effort ...] [--old-skill-dir <path>]`.
   - **What it runs.** Expands evals × configurations × `runs_per_configuration` from the runbook (and every matrix combo, see Runbooks below), schedules the with-skill and baseline runs of each eval **interleaved in the same batch** — never with-skill first and baselines later — with bounded worker parallelism (`--workers`, default 2), and executes each run hermetically per the sandbox contract above.
   - **Sandbox base.** Before the first run in each workspace it mints one runnable base per configuration at `sandbox-base/<configuration>/` (write-once; schemas.md § Workspace layout). Every run copies its configuration's base into a fresh temp sandbox and layers that eval's `files[]` on top — the base is both the as-run content record and the ground every run provably shares.
   - **Harness failures.** A run that errors, times out, or hits a missing fixture is a harness failure, never a target result: it is quarantined under `harness-failures/` with a note of what broke (exit code 2 signals at least one occurred), and is never counted toward either config's pass rate.
   - **Repairing one run.** Re-run a single failed run with `eval-skill run-behavioral --cli=<cli> --runbook ../../runbooks/<id>.yaml --repo-root=../.. --eval-id N --configuration <name> --workspace ../../result/<id>/iteration-N --run-number M --skill-dir <path>` — the same driver `run-iteration` calls per run, also usable directly. `--skill-dir` is required for `with_skill`/`new_skill`/`old_skill` (main() rejects those configurations without it rather than silently running a skill-less repair under the with-skill label); omit it only for `without_skill`. Under a runbook declaring `executor: subagent`, run-behavioral serves only the baseline arms (`without_skill`/`old_skill`): a primary-arm configuration is refused — that carrier runs it as an in-session subagent — unless `--override-executor` forces a hermetic run (forced results belong to the cli stratum).
   - **Output.** Progress prints to stderr as each run finishes; a summary JSON (run statuses, in queue order) prints to stdout.
4. `events.jsonl`, `transcript.md`, `metrics.json` (under `outputs/`), and `timing.json` are now written mechanically by the runner as part of step 3 — the agent no longer hand-captures them. The "save timing the moment it arrives" discipline applies wherever a subagent executes the run instead — `executor: subagent`, or a no-CLI fallback: its task notification carries `total_tokens`/`duration_ms` and is the ONLY place that data exists, so capture it as each notification arrives or it is gone.

### Grade

Grade every run with a fresh, blind grader following `agents/grader.md`: binary pass/fail per assertion with cited evidence, burden of proof on the expectation, superficial compliance fails. **Preferred (headless CLI):** run `eval-skill grade-result --cli=<claude|codex> run <iteration-dir>`. One fresh `claude -p` / `codex exec` process per run writes its `grading.json`, including the concrete grader `cli`; a grader process that times out or returns unparseable output is a harness failure, never a target verdict. Fall back to a grader subagent per run only without a CLI. Programmatically checkable assertions get a script, not eyeballing.

Prefer a different model for grading than the executor used (pass `--model`/
`--effort` to `grade-result`, or pin them once as the runbook's `grader`
field — `{model, effort}`, schemas.md § runbook — so every `run` and
`review` call picks them up without repeating the flags; an explicit
`--model`/`--effort` still overrides its own key independently). Same-model
grading is a known leniency bias — when unavoidable, disclose it in the
report (quality-standards E11). The E10 known-good/known-bad grader fixture
check matters more, not less, when the grader model changes.

Process expectations that reduce to command patterns (ran X, never ran Y, X
before Y, at most N commands) get `eval-skill check-trace` against the run's
`events.jsonl` — deterministic, reproducible, and the failure evidence names
the exact command; reserve the LLM grader for judgments a regex can't make.

Output contract: `grading.json` (exact fields `text`, `passed`, `evidence`).

The grader also meta-critiques the eval set (`eval_feedback`): assertions a clearly-wrong output would also pass, and outcomes no assertion covers. Feed these back into the eval suite — a passing grade on a weak assertion is worse than useless.

### Aggregate and analyze

```bash
eval-skill aggregate-benchmark <workspace>/iteration-N --skill-name <name> --cli=<claude|codex> --executor-model <model> --effort <effort>
```

- **Identity flags.** Pass `--executor-model`/`--effort` — the same values given to `run-iteration`/`run-behavioral` for that combo; omit whichever one the combo didn't set. Without them, `benchmark.json`'s `metadata.executor_model`/`executor_effort` stay null and a matrix run's `matrix.md` identity columns are permanently em-dash. `run-iteration`'s summary JSON prints the exact invocation as `aggregate_command`, per combo, ready to copy-paste.
- **Output.** Produces `benchmark.json`/`benchmark.md`: per-configuration stats with raw counts, and the primary-minus-baseline delta (resolved by configuration name).
- **Analyzer pass.** Then run an analyzer per `agents/analyzer.md` to surface what aggregates hide: non-discriminating assertions (pass in both configs), always-fail assertions, skill-hurting cases (fail with skill, pass without), and flaky high-variance evals. The analyzer reports patterns and may label a suspected failure layer (target/harness/environment/etc.) when it has independent evidence for it — it never suggests skill improvements.

A skill that bundles `references/` or `scripts/` needs at least one eval that requires reaching them, asserting the agent actually did — a reference nobody consults or a script nobody runs is dead weight the pyramid otherwise can't detect.

### Human review before self-evaluation

Generate the review viewer BEFORE forming your own judgment of the outputs — the human sees results first, so the model can't pre-rationalize them:

```bash
eval-skill generate-review <workspace>/iteration-N \
  --skill-name <name> --benchmark <workspace>/iteration-N/benchmark.json
```

`generate-review` serves a bundled viewer template (Outputs tab with per-case feedback boxes, Benchmark tab with the stats; `--previous-workspace` adds last iteration's outputs and comments for regression spotting). Headless environments: `--static <output.html>` remains a fallback for feedback collection without a viewer server — it writes a standalone file, never persisted into the workspace by the default flow — and feedback still downloads as `feedback.json`. Read `feedback.json` when the user finishes — empty feedback means that case looked fine. `render-report` complements the viewer mechanically: the viewer collects human feedback; `report.html` is the persistent, read-only results page.

### Reporting

What the human-facing report must state — versions, raw counts, the delta and whether the lift justifies its cost, failures by root cause, hypotheses labeled as hypotheses — is normative and lives in `references/quality-standards.md` § Reporting requirements.

## Tier 3: Pressure testing

For discipline-enforcing skills. Write 3+ scenarios combining 3+ pressures each (taxonomy, scenario requirements, and the setup preamble: `references/scenario-design.md` § Pressure scenarios). Run them through the same paired-run harness as Tier 2 — pressure evals are `type: pressure` entries in the runbook's eval suite, persisted for regression like everything else.

For every violation, capture rationalizations **verbatim** in the rationalization-capture format (scenario-design.md § Rationalization capture) — create-skill consumes that artifact to harden the skill. When an agent read the skill and still violated, run the meta-testing protocol (scenario-design.md § Meta-testing) to classify the fix: foundational principle, wording, or organization.

Bar: every scenario held, and the final iteration surfaced no new rationalizations.

## Blind A/B comparison (optional)

For "is the new version actually better?" between two skill versions when assertions can't settle it:

1. Run both versions on the same evals.
2. Spawn a comparator per `agents/comparator.md` on unlabeled outputs — twice per pair, once in each presentation order; the comparator never knows which ordering it sees.
3. Combine per `references/schemas.md` § comparison.json: categorical verdicts (A/B/tie), agreement of both orderings, disagreement → `inconsistent` (no evidence); either ordering breaking the comparison protocol itself (inaccessible output, exposed identity) → `inconclusive`, distinct from `inconsistent`.
4. Optionally spawn an analyst per `agents/analyst.md` to unblind and convert the verdict into prioritized, categorized improvement suggestions — that output is what create-skill's REFACTOR step consumes.

Judgment and diagnosis are deliberately separate agents: the comparator measures, the analyst explains, the analyzer (benchmark mode) pattern-checks. Don't collapse them.

## Regression mode

The reason eval-skill exists independently: a shipped skill's suite must be re-runnable long after creation.

**When:** after any skill edit; after a model upgrade; periodically for load-bearing skills.

1. Load the target's runbook from the repo's `runbooks/` directory — the suite lives in the repo, never inside the skill directory (a portable standalone `evals.json` with the same fields is the fallback for a target outside a runbooks-style repo). Runbooks and `history.json` must use schema v2; v1 is unsupported by this revision and has no compatibility path. On a missing or mismatched value, stop and reconcile the data against `references/schemas.md` before trusting the suite or history.
2. Re-run the applicable tiers (mechanical edit → Tier 0 + existing suite re-run; content edit → Tier 2; description edit → Tier 1). Baseline for an edit is the pre-edit snapshot.
3. Compare only against the last-known-good entry with the same concrete `(cli, model, effort, executor)` quadruple in the workspace's `history.json`, then append a `"kind": "regression-check"` entry recording all four (contract: `references/schemas.md` § history.json). Never skip the append — an unwritten history is why regressions go unnoticed.
4. **Retirement check** after model upgrades: re-run the baseline configuration too. A baseline that now passes means the model learned what the skill teaches — flag the skill for slimming or retirement rather than celebrating the pass rate.

Verdicts vs last-known-good: `held` (within noise), `regressed` (below by more than noise — report raw counts, then fix the skill, never the eval), `improved`, or `inconclusive` (harness failures made the comparison unusable — repair and re-run before drawing any of the other three). An aggregate `improved` never excuses a P0 eval flipping to fail — a per-eval regression is checked and reported on its own, not averaged away.

## Runbooks: saved eval runs

In a repo that mirrors the skill-forge layout — canonical `skills/` mounted into `.claude/skills` and `.agents/skills`, plus `runbooks/` and `result/` — an eval run is defined once and re-executed forever:

- **`runbooks/<id>.yaml`** freezes the run definition AND the eval suite itself (`evals[]`, `trigger_evals[]`): target skill, tiers, CLI execution mode, repetitions, configurations, release gates (contract: `references/schemas.md` § runbook). Its `executor` field declares the carrier — `cli` or `subagent`; the concrete platform is deliberately selected at execution time, not stored in the runbook. The suite lives here — never inside the target skill directory. A top-level `runbooks/<id>.yaml` is reserved for skills the repo actually ships (its `target_skill` never points into the `runbooks/fixtures` directory).
- **`result/<id>/`** is the workspace root for that runbook, replacing the sibling `<skill-name>-workspace/` convention; everything beneath it follows the standard workspace layout.
- **The `runbooks/fixtures` directory** is exercise material — sample skills, planted eval sets, and fixture-targeting runbooks (saved as `runbooks/fixtures/<name>-runbook.yaml`, never as a top-level id). A fixture-targeting run is **ephemeral by contract**: execute it from a workspace outside the repo (e.g. under `mktemp -d`), then keep any evidence worth keeping as a snapshot under the *invoking* runbook's iteration dir, at `result/<id>/iteration-N/fixture-exercises/<slug>/` — the CLI refuses a repo-internal workspace or `--out` for a fixture target (schemas.md § runbook). `fixture-exercises/` snapshots and runs swept into a run's `outputs/_workdir` bucket are records, never gradeable runs — grading discovery skips both, exactly like `harness-failures/` and `sandbox-base/`.

### Model × effort matrix

A runbook can declare `matrix:` — a list of `{model, effort, cli}` combos to sweep in one run; without one, a run uses whatever single model/effort the `--model`/`--effort` CLI flags set. Entry semantics (cli-scoping, skip rules), the flag-vs-matrix conflict rule, and slug validation: `references/schemas.md` § runbook.

- **Per-combo workspaces.** `run-iteration` runs combos one at a time, each into its own nested `iteration-K/<slug>` directory with its own `benchmark.json`/`review.json`.
- **Merged view.** `eval-skill matrix-report --workspace-base <workspace>/iteration-K` merges every combo's `benchmark.json`/`review.json` into one `<workspace-base>/matrix.md` table.
- **Regression strata.** With a matrix, the comparison key becomes **(cli, model, effort, executor)** — verdicts only compare against a `history.json` entry matching all four.

### Choreography for "run the runbook"

1. **Dispatch a fresh subagent to execute the runbook end-to-end.** The subagent reads the runbook and follows this SKILL.md tier by tier; the calling context stays clean, reviews the report, and never leaks hypotheses into runs.
   - **Inline alternative — what unlocks concurrency.** When the calling context is already dedicated to this run (an author iterating on their own skill), run the tiers directly instead; never for a shipping gate, where clean context is the point. A dispatched run nests inside a live CLI session and keeps `--workers 2`; inline, nothing is nested, so pass `--workers 4`+ to both `run-iteration` and `grade-result run`. That default protects the nested case; 2 is not a measured ceiling.
2. Validate `schema_version: 2` and read `executor` — `cli` or `subagent`, nothing else. Then bind the concrete CLI per Tier 1's platform-flag rule, freeze it for the whole iteration, and pass its literal flag to every script below — **both** carriers need it, since `subagent` still drives its baseline arm and every grader through the CLI.
   **Mechanical pipeline (`executor: cli`):** `eval-skill run-runbook --cli=<claude|codex> --runbook ../../runbooks/<id>.yaml --workspace ../../result/<id>/iteration-N` chains steps 3–5's Tier 2/3 half — `--help` documents flags and exit semantics; per-step commands below stay authoritative for `executor: subagent` and debugging.

3. **Run the tiers the runbook lists** (paths below are relative to the skill root, per Tier 0):
   - Tier 0: `eval-skill validate-skill` on the target.
   - Tier 1: `eval-skill run-eval --cli=<claude|codex> --runbook ../../runbooks/<id>.yaml --skill-path ../../<target_skill> --out ../../result/<id>/iteration-N/trigger_results.json`. It reads `trigger_evals[]`/`target_skill` straight from the runbook — but state `--skill-path` explicitly (the default-resolution trap, Tier 1 above).
   - Tier 2/3, `executor: cli`: `eval-skill run-iteration --cli=<claude|codex> --runbook ../../runbooks/<id>.yaml --repo-root=../.. --workspace ../../result/<id>/iteration-N` — one command per iteration; what it expands and how: Tier 2 § Run, step 3.
   - Tier 2/3, `executor: subagent`: fan the with-skill runs out as subagents in ONE turn; drive each baseline run through `run-behavioral` so the baseline stays hermetic. You are hand-building run dirs the runner would otherwise write — follow `references/execution-modes.md`'s run-dir contract exactly; its traps fail the whole iteration, not one run.
4. **Grade:** `eval-skill grade-result --cli=<claude|codex> run ../../result/<id>/iteration-N --runbook ../../runbooks/<id>.yaml` — every run gets a fresh headless CLI grader, `--workers` at a time (default 2).
   - `--runbook` is what lets the grader also emit each run's `expected_output_comparison`: the eval's `expected_output` lives only in the runbook (`eval_metadata.json` deliberately omits it so the executor stays blind). Omit the flag and grading still works, but the comparison block never appears.
   - For a matrix runbook, this one call already covers every nested combo: `grade-result run`'s directory walk is fully recursive from the given root, so grading `iteration-N` finds every combo's `<slug>/.../run-M` dirs beneath it in the same pass — no per-combo invocation needed.
5. **Aggregate, review, and report:**
   - Aggregate: `eval-skill aggregate-benchmark ../../result/<id>/iteration-N --skill-name <name> --cli=<claude|codex> --executor-model <model> --effort <effort>` (identity-flag rules: § Aggregate and analyze; `run-iteration`'s summary JSON prints each combo's exact `aggregate_command`).
   - Human review (optional, on demand — not a workspace artifact the default flow writes): `eval-skill generate-review` serves the interactive viewer; read `feedback.json` if the user reviews through it.
   - Verdict: `eval-skill grade-result --cli=<claude|codex> review ../../result/<id>/iteration-N --runbook ../../runbooks/<id>.yaml`. The gate verdict (pass / fail / inconclusive) is computed mechanically from `benchmark.json`; the headless reviewer contributes only the diagnosis and improvement suggestions (hypotheses), written to `review.json` + `review.md`.
   - Matrix runbooks: repeat aggregate + review per combo — each targets its nested `../../result/<id>/iteration-N/<slug>` dir directly, never the bare `iteration-N` root, which would mix every combo's models into one aggregate — then merge with `eval-skill matrix-report --workspace-base ../../result/<id>/iteration-N`; the runbook-level verdict is pass only if every executed combo passes.
   - Report: the Verdict step above already rendered `report.html` next to `review.json` automatically (`review --no-render` opts out — what `run-runbook` passes, rendering once at the iteration root itself), so a graded iteration ends with a report on every carrier. `eval-skill render-report --workspace-base ../../result/<id>/iteration-N --runbook ../../runbooks/<id>.yaml` re-renders on demand — one self-contained page combining the matrix summary (best lift/time/tokens/cost per combo, per-eval × combo lift grid, trigger results) with per-combo detail (runbook prompt + expected_output vs actual outputs, paired runs, per-run expectations, review verdict + suggestions).
6. Append the iteration to the workspace's `history.json`, including the concrete `cli`, `model`, and `effort` — a runbook run is a regression-mode entry like any other.

A Claude CLI run and a Codex CLI run of the same runbook are two separate result strata, never pooled or used as each other's regression baseline. Run the runbook once from each host when the skill ships to both platforms.

## Eval integrity rules

Non-negotiable, in every tier:

- **Agent fails the eval → fix the skill, never weaken the eval.**
- **An eval that passes without the skill measures nothing.** Flag it as non-discriminating; replace it.
- **Grade behavior and artifacts, not recitation.** A transcript that quotes the skill but produces the wrong output is a FAIL. See the grader-context policy in `agents/grader.md`.
- **Report raw counts.** Mean ± stddev on n < 5 is decoration. A delta within noise of the bar is "no evidence", not a pass (full small-N rules: `references/quality-standards.md` § Shipping bar).
- **Fixtures mirror reality.** A minimal input crafted so assertions happen to pass is the eval-side version of testing a mock.
- **Executors are blind.** Never include, or direct the executor toward, assertions/expected outputs/hypotheses/sibling-run artifacts (including the target's runbook — it carries the expectations).
- **A harness/infrastructure error is neither a pass nor a fail.** Repair and re-run; if unrepairable, the verdict is `inconclusive` and is never counted in a pass rate or a delta.
- **Every spawned run counts.** Re-running until an output passes and discarding the rest is fabrication.
- **Read-only default.** Never edit the target skill while an evaluation is in flight — a mid-run edit invalidates every paired comparison already spawned. Revision belongs to create-skill; the only exception is the description-optimization loop, and only when the user asks for it.

## Platform degradation

The primary path on any platform is the headless CLI driver (`claude -p` / `codex exec` — same abstraction, one driver each; see Bundled files for where they live). Degrade only when a capability is missing.

**This table is capability loss, not the runbook's `executor` field** — orthogonal concerns. `executor: subagent` is a speed choice made while the CLI is available; the subagent column below is the fallback for when it is not. Neither licenses the other.

| Capability | CLI available (Claude Code / Codex) | No CLI, subagents available | No subagents (Claude.ai) | No display/headless |
|---|---|---|---|---|
| Tier 0 | subcommand | subcommand | subcommand | subcommand |
| Tier 1 | run-eval (explicit `--cli=<claude|codex>`) | subagent trials | skip measurement; qualitative description review only | run-eval or subagent trials |
| Tier 2/3 | paired fresh CLI processes | paired subagents | run test prompts inline yourself, sequentially; skip baseline/benchmark; rely on human review | paired fresh CLI processes |
| Grading | grade-result (fresh CLI grader per run) | grader subagents | grade inline and disclose the author-as-grader conflict | grade-result |
| Review | viewer server | viewer server | present outputs in conversation | viewer `--static`; `report.html` |

Each degradation step weakens isolation — CLI processes can't see the conversation, subagents inherit the harness's skill registry, and inline self-testing makes the author the executor. Disclose the execution mode when reporting results.

## Bundled files

- `bin/eval-skill-<platform>` — compiled CLI binary, present only in a published package (§ CLI resolution; a dev checkout has no `bin/` and runs the repo's TypeScript source under Bun)
- `agents/grader.md` — assertion grader contract (binary, evidence-cited, meta-critique)
- `agents/comparator.md` — blind A/B judge (categorical verdicts, single-ordering-per-invocation)
- `agents/analyst.md` — post-hoc unblinded improvement analysis
- `agents/analyzer.md` — benchmark pattern analysis (measurement only)
- `references/execution-modes.md` — the `cli` / `subagent` carrier contracts and the subagent run-dir rules
- `references/quality-standards.md` — the normative rulebook with rule IDs and the shipping bar
- `references/scenario-design.md` — scenario doctrine for all tiers
- `references/schemas.md` — JSON contracts and the workspace layout
- `assets/eval_review.html` — trigger-query curation template

### `eval-skill` subcommands

- `validate-skill` — Tier 0 lint
- `init-runbook` — scaffolds a starter `runbooks/<id>.yaml` for an existing skill (mounts are `create-skill validate-mounts --fix`'s job instead; see the `init-eval-infra` skill, which choreographs both)
- `run-eval` — real-harness trigger measurement (headless CLI, explicit `--cli=codex` or `--cli=claude`; consumes `--runbook` or `--eval-set`; `--out PATH` persists the result as `trigger_results.json`)
- `freeze-inputs` — cross-platform input-hash manifest (`write`/`check`; shasum-compatible)
- `run-iteration` — Tier 2/3 batch orchestrator: one command per iteration, interleaved with-skill/baseline pairing, model×effort matrix expansion; refuses a runbook declaring `executor: subagent` (it is the cli carrier — `--override-executor` forces the batch), as does `run-runbook`
- `run-behavioral` — hermetic single-run Tier 2/3 executor (sandbox population, copy-back, harness-failure quarantine); what `run-iteration` calls per run, also usable directly to repair or re-run one run
- `grade-result` — headless CLI grading (`run`: grading.json per run, `--workers N` graders in parallel; `--runbook PATH` adds each run's `expected_output_comparison`) and gate verdict + improvement suggestions (`review`: review.json/review.md, gates from a runbook)
- `check-trace` — deterministic process assertions over a run's `events.jsonl` (`--ran/--not-ran/--order/--max-commands`)
- `run-loop`, `improve-description`, `generate-report` — description-optimization loop
- `aggregate-benchmark` — benchmark aggregation
- `matrix-report` — merges a matrix run's per-combo benchmark.json/review.json files into `<workspace-base>/matrix.md`
- `render-report` — renders a runbook iteration's benchmark.json, review.json, per-run timing.json/grading.json, and trigger_results.json (plus the runbook) into one self-contained `<workspace-base>/report.html`; `grade-result review` runs it automatically after its verdict (`--no-render` opts out)
- `generate-review` — human review UI server

Executor drivers (`claude -p`/`codex exec` process wrappers each subcommand calls into) live in the repo's src/eval-skill source tree (wired at src/eval-skill/composition/executors), not inside this skill directory.
