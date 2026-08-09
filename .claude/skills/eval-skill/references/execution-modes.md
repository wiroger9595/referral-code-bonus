# Execution modes

How a Tier 2/3 run is actually carried out. The runbook's `executor` field picks the carrier; this file is the contract for each one.

Read this when a runbook says `executor: subagent`, when a run's artifacts look wrong, or when deciding which carrier a given question deserves. The gate rule that governs the choice lives in `quality-standards.md` § Execution-carrier rules — **a subagent-executed iteration may never close a gate.**

## In-vivo vs in-vitro

Tier 1 trigger measurement runs the real CLI and measures the production harness's actual discovery decision (in-vivo) — in a throwaway sandbox of its own, where the candidate is the only project-level skill and the user-level layer still loads, so the decision field stays realistic without the run ever writing into the repo's discovery roots. Tier 2/3 behavioral and pressure runs want the opposite — a controlled comparison where the only variable between configurations is whether the target skill is present (in-vitro).

Neither is an accident: Tier 1 needs the real harness, Tier 2/3 needs an isolated one. The two carriers below differ precisely in how much of that isolation they can actually deliver.

## `executor: cli` — hermetic, the gating carrier

Every run gets its own sandbox under the OS temp dir (`mkdtempSync`), never inside the repo, so the executor process cannot see the repo's mounted skills, `CLAUDE.md`, or plugins. Driven by `run-iteration` / `run-behavioral`; all artifacts are written mechanically.

- **Isolation flags.** `claude` runs pin `--setting-sources project --strict-mcp-config`; `codex` runs use `--ephemeral -s workspace-write --skip-git-repo-check -C <sandbox>` — `--ephemeral` is precisely the no-persistence flag hermetic execution requires.
- **Symmetry.** Both configurations get identical CLI flags; the only difference is whether `.claude/skills/<name>/` (`.agents/skills/<name>/` for codex) exists in the sandbox. The pairing is structurally symmetric but not behaviorally identical: mounting a skill turns on the CLI's skills subsystem (its built-in skills become listed) while an empty mount dir never does — platform overhead the with-skill side alone carries.
- **Copy-back.** `events.jsonl`, `transcript.md`, `metrics.json`, `timing.json` are copied back into the workspace after the process exits, then the sandbox is removed.
- **Blinding.** Enforced by construction: the runbook and its expectations never enter the sandbox.

## `executor: subagent` — fast, iteration-only

Runs execute as subagents inside the current session rather than as sandboxed CLI processes. This is what a scaffolded runbook ships with, alongside `runs_per_configuration: 1`, and both dials tighten together only for a gate-closing certification run (quality-standards.md § Shipping bar's evidence levels) — the authoring loop may ship without one.

**Asymmetric pairing, by design.** The two arms do not use the same carrier:

| Arm | Carrier | Why |
|---|---|---|
| `with_skill` | subagent, in-harness | speed — no sandbox setup, no CLI cold start, all runs fan out in one turn |
| `without_skill` (baseline) | hermetic CLI | a subagent baseline is **not skill-free** — it inherits this session's skill registry, including the target skill itself |

That asymmetry is the price of a trustworthy baseline, and it has a consequence that must be stated in every report built on this carrier: the with-skill arm runs in a richer environment than the baseline, so **the measured lift is an upper bound, not a clean delta**. It answers "did that edit move things" — never "how much is this skill worth".

The CLI enforces this split: `run-behavioral` refuses a primary-arm configuration (`with_skill`/`new_skill`) under a subagent runbook, and `run-iteration`/`run-runbook` refuse such a runbook whole — silently promoting one arm (or the whole batch) to the hermetic carrier is exactly the substitution that turns a cheap iteration into an unasked-for cli run. `--override-executor` forces the hermetic run; forced results belong to the cli stratum, never comparable with subagent-carrier numbers.

**What is structurally unavailable here:**

- **`events.jsonl`** — the headless CLI's own stdout event stream has no subagent equivalent. Every `check-trace` process assertion (`--ran` / `--not-ran` / `--order` / `--max-commands`) is therefore unavailable, and must be re-expressed as an LLM-graded expectation or dropped. `check-trace` itself fails safe: a missing or unparseable events file exits 2 as a harness failure, never a silent pass — so the risk is not a false green, it is quietly losing a deterministic check and not saying so in the report.
- **Blinding by construction** — a subagent inherits the parent's context, so blinding becomes a discipline: pass the eval's `prompt` and nothing else. Never the runbook, expectations, `expected_output`, hypotheses, or a sibling run's artifacts.
- **Filesystem containment** — runs execute in the real working directory, so scenario-design.md's standing "writes nothing outside its own `outputs/`" assertion neither holds nor protects anything. Expect scratch files.

### Run-dir contract (what the subagent path must produce)

The downstream pipeline — `grade-result`, `aggregate-benchmark`, `generate-review`, `render-report` — discovers runs by walking the workspace layout and does not care who produced them. Satisfy the layout and nothing downstream changes. `schemas.md` § Workspace layout is normative; these are the parts a hand-driven path gets wrong:

- **Directory name must be `run-<base10 int>`.** `aggregate-benchmark` parses it and throws `AggregationError` on failure — that kills the whole iteration, not one run.
- **The eval directory name must equal the runbook's `eval_name` exactly.** No slugging, no sanitization: `render-report` reconstructs paths from the runbook, not from `benchmark.json`.
- **`grading.json`'s `grader.cli` must match the `--cli` passed to `aggregate-benchmark`.** A mismatch is also a whole-iteration `AggregationError`.
- **`metrics.json` goes in `<run-dir>/outputs/`, not the run-dir root.** At the root it is silently ignored and `execution_metrics` degrades to zeros rather than erroring.
- **Write every `eval_metadata.json` before fanning out, sequentially.** It is write-once by design, so concurrent first-writers race. It carries `eval_id`/`eval_name`/`type`/`prompt`/`expectations`/`files` — and deliberately **not** `expected_output`, which would leak into the grader's blind input.
- **Append the same prompt suffix the hermetic runner uses** ("Write every file you produce under the `outputs/` directory of your current working directory"). Omit it and `outputs/` stays empty, which cascades: no `files_created`, the viewer stops recognizing the directory as a run, and the grader starts returning `inconclusive`.
- **A failed run is not a result.** Quarantine it under `harness-failures/` with a note; never leave a partial run dir in place, and never count it toward either arm's pass rate.
- **Capture timing as each notification arrives.** A subagent's task notification carries `total_tokens`/`duration_ms` and is the ONLY place that data exists — unlike the CLI path, nothing writes `timing.json` mechanically.

### Reporting and history

- State the carrier in the report, next to the CLI and model. A reader cannot infer isolation from a pass rate.
- `history.json` entries carry `executor` as part of the `(cli, model, effort, executor)` regression-stratum key. A subagent iteration is never compared against a `cli` iteration — that would manufacture a `held`/`regressed` verdict out of a difference in isolation.
