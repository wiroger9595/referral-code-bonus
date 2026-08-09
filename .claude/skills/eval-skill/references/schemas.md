# JSON Schemas and Workspace Layout

The data contracts of the eval pipeline. Field names are load-bearing — the viewer and the aggregation script read them exactly. When generating any of these files manually, reference this document. Throughout: a `cli` field always carries the concrete platform — `"claude"` or `"codex"` — never the literal string `"cli"`, which is a valid value only of *carrier* fields (`executor`).

These schemas are also the interface between create-skill and eval-skill: create-skill writes the runbook (run definition + eval suite); eval-skill consumes it and produces the rest.

## Contents

- [Workspace layout (normative)](#workspace-layout-normative)
- [runbook (YAML)](#runbook-yaml)
- [Eval suite fields](#eval-suite-fields)
- [eval_metadata.json](#eval_metadatajson)
- [grading.json](#gradingjson)
- [metrics.json](#metricsjson)
- [timing.json](#timingjson)
- [benchmark.json](#benchmarkjson)
- [comparison.json](#comparisonjson)
- [analysis.json](#analysisjson)
- [history.json](#historyjson)
- [feedback.json](#feedbackjson)
- [review.json](#reviewjson)
- [trigger_results.json](#trigger_resultsjson)

## Workspace layout (normative)

Results live in `<skill-name>-workspace/`, a sibling of the skill directory — or, when the run is defined by a runbook in a repo with `runbooks/` and `result/` directories (the skill-forge mirror), at `result/<runbook-id>/` instead. Either way the structure below the workspace root is identical, and it is the single normative layout — `aggregate-benchmark` and the viewer discover runs by walking it:

```
<skill-name>-workspace/
├── skill-snapshot/                  # baseline copy when improving an existing skill
├── history.json                     # version lineage across iterations (regression mode)
└── iteration-N/                     # one dir per iteration; a matrix combo nests INSIDE it as iteration-N/<slug>/
    ├── input-hashes.sha256          # sha256 manifest (freeze-inputs), frozen before this iteration's first run
    ├── run-iteration-summary.json   # optional: run-iteration prints this summary to stdout; present only when the executing agent saves it here
    ├── feedback.json                # human review output (written by the viewer)
    ├── matrix.md                    # matrix runbooks only: matrix-report's cross-combo table
    ├── report.html                  # self-contained runbook report (auto-rendered by grade-result review)
    ├── trigger_results.json         # run-eval --out: this iteration's Tier 1 results
    ├── sandbox-base/                # non-matrix: one runnable sandbox base per configuration (run-iteration; write-once)
    │   └── <configuration>/<name>/  # keyed by configuration — old_skill/new_skill usually share a basename
    │       └── binaries.sha256      # sha256 of excluded platform binaries; present only if any were excluded
    ├── benchmark.json               # non-matrix: aggregated stats for this iteration
    ├── benchmark.md
    ├── review.json                  # non-matrix: gate verdict + suggestions (grade-result review; runbook runs)
    ├── review.md
    ├── harness-failures/            # non-matrix: quarantined evidence from runs that errored/timed out — never a target result
    ├── fixture-exercises/           # reserved: snapshots of fixture-exercise evidence (SKILL.md § Runbooks) — records, never gradeable runs
    ├── <eval-name>/                 # non-matrix: one dir per eval, descriptively named
    │   ├── eval_metadata.json       # {eval_id, eval_name, prompt, expectations[]}
    │   └── <configuration>/         # with_skill | without_skill | new_skill | old_skill
    │       └── run-M/
    │           ├── events.jsonl     # raw event stream (Tier 2/3 hermetic runs; run-behavioral)
    │           ├── transcript.md    # rendered transcript
    │           ├── timing.json
    │           ├── grading.json
    │           └── outputs/         # files the run produced, incl. metrics.json; strays land in outputs/_workdir/
    └── <slug>/                      # matrix runbooks only: one dir per {model, effort, cli} combo — same shape as the non-matrix entries above (sandbox-base/, benchmark.json/md, review.json/md, harness-failures/, <eval-name>/...), scoped to this combo alone
```

The `iteration-K@<slug>` sibling layout is retired; this revision reads only the nested `iteration-K/<slug>` layout described above — a matrix combo workspace is always a CHILD of its iteration dir, never a sibling of it.

Configuration names are meaningful: `with_skill` vs `without_skill` for new skills, `new_skill` vs `old_skill` for improvements. The aggregator resolves primary/baseline by these names, never by directory order.

`input-hashes.sha256` is produced by `eval-skill freeze-inputs write` (SKILL.md § Tier 2 Run) covering the target skill directory and every fixture file, written once before the first run and re-verified with `eval-skill freeze-inputs check` after runs complete — a `MISMATCH`/`MISSING` line means an input changed mid-iteration and the iteration is compromised. The manifest format itself is unchanged and stays `shasum -c`-compatible: one `<sha256-hex>  <POSIX-relative-path>` line per file.

`sandbox-base/` is both the as-run record and the thing that actually runs. `run-iteration` mints it into each iteration workspace before its first run — `<configuration>/<discovery-root>/skills/<skill-dir-name>/`, one base per configuration, write-once (never overwritten by a later invocation into the same workspace). Every run then copies its configuration's base into a fresh temp sandbox and layers that eval's declared `files[]` on top, so every run in an iteration starts from provably identical ground and nothing is assembled twice.

Keyed by configuration, not just skill name, because `old_skill` and `new_skill` are two different directories that usually share a basename. Only the discovery root the running CLI reads is minted (`.claude` for claude, `.agents` for codex) — carrying both would double the copy and feed `copyBack`'s stray sweep an entire mounted skill for a root the run never touches.

A baseline configuration mints its discovery root EMPTY, and its runs additionally pass `--safe-mode`. Either alone would do less than it appears: safe-mode stops the skill being *loaded* but not *read*, so an agent exploring its cwd could still open `SKILL.md`; an empty root alone leaves the harness responsible for having plugged every other leak. Together the control half is guaranteed on both sides.

Distinct from the workspace-root `skill-snapshot/`, which is the manually-created pre-edit baseline used by the improvement flow.

The current host's own per-platform binary is KEPT in the base; every other platform's is dropped. The base is the runtime, not provenance — a mounted skill whose § CLI resolution finds no binary is a skill that cannot run its own CLI inside the sandbox, and an eval measuring that is measuring the wrong thing. Off-platform binaries are dead weight there.

Because the base carries real binaries it can be large. Whether it belongs in version control is the consuming repo's decision, not this skill's — decide it in your own `.gitignore`.

`fixture-exercises/` is the reserved snapshot dirname for fixture-exercise evidence (SKILL.md § Runbooks): a fixture-targeting run executes in a temp workspace outside the repo, and whatever is worth keeping is copied here, under the invoking runbook's iteration dir. Grading discovery (`grade-result run`'s recursive `run-*` walk) treats five path segments as records rather than gradeable runs and never descends results out of them: `harness-failures/`, `sandbox-base/`, `skills-as-run/` (the pre-`sandbox-base` record, still skipped so older iterations stay readable), `fixture-exercises/`, and `_workdir` (the `outputs/_workdir/` stray sweep, which can carry an agent-under-test's own nested eval workspace wholesale). A snapshot may therefore contain complete nested `iteration-*/.../run-M` trees without ever polluting the enclosing iteration's grading or aggregation.

## runbook (YAML)

A saved eval-run definition at `runbooks/<id>.yaml` (repo root), executed by the Runbooks choreography in SKILL.md. The runbook carries BOTH the run configuration AND the eval suite itself (`evals[]`, `trigger_evals[]`) — the suite lives here, in the repo, never inside the target skill directory (extra files inside a skill alter discovery and packaging, and the repo owns its eval history). Freezing the definition in one file is what makes the run repeatable across sessions, models, and platforms.

**Parser note:** runbook YAML is parsed with `Bun.YAML` (YAML 1.2), not the previous PyYAML (YAML 1.1) — the one behavioral divergence that matters here is bare, unquoted `yes`/`no`/`on`/`off` (in any casing): YAML 1.1 resolves these to booleans, YAML 1.2 leaves them as plain strings. Always write booleans as `true`/`false` in runbook YAML (`executor: cli`'s neighboring fields, `require_lift`, etc.) — never rely on a bare `yes`/`no`/`on`/`off` being reclassified.

```yaml
schema_version: 2
runbook: eval-skill
target_skill: skills/eval-skill
tiers: [0, 1, 2]
configurations: [with_skill, without_skill]
executor: cli
runs_per_configuration: 3
gates:
  min_pass_rate: 0.8
  require_lift: true
result_root: result
evals:
  - id: 1
    eval_name: descriptive-name
    type: behavioral
    prompt: User's task prompt
    expected_output: Description of expected result
    files: []
    expectations:
      - The output includes X
      - The skill used script Y
trigger_evals:
  - {query: realistic user prompt, should_trigger: true}
  - {query: near-miss prompt, should_trigger: false}
```

**Run-configuration fields:**
- `schema_version`: must be `2`. This revision does not accept v1 runbooks and provides no migration or compatibility path; reconcile the runbook against this contract before running it
- `runbook`: the id; the workspace root for this runbook is `<result_root>/<runbook>/`
- `target_skill`: repo-root-relative path to the canonical skill directory (`skills/<name>`, never a `.claude/skills/...` or `.agents/skills/...` mount path). A top-level `runbooks/<id>.yaml` never targets `runbooks/fixtures/...`: a fixture-targeting runbook is exercise material, saved as `runbooks/fixtures/<name>-runbook.yaml` and run under SKILL.md § Runbooks' fixture-exercise contract (temp workspace outside the repo; evidence persisted only as a `fixture-exercises/` snapshot inside a real iteration dir — `run-iteration`/`run-behavioral` refuse a repo-internal workspace for such a target, `run-eval` a repo-internal `--out`)
- `tiers`: which pyramid tiers this run executes
- `configurations`: the paired configuration names (same vocabulary as the workspace layout)
- `executor`: the Tier 2/3 carrier — `"cli"` or `"subagent"`, required, no default. `"cli"` means every run executes hermetically in its own sandbox; `"subagent"` means the with-skill arm fans out in-harness while the baseline arm stays hermetic (contracts: `execution-modes.md`). Concrete platform names (`"claude"`, `"codex"`) are invalid here and a mapping is invalid too — this field names a carrier, not a platform and not a per-phase policy. `loadRunbook` validates but never normalizes it, so a runbook round-tripped through it still matches its own YAML. Tier 1 measurement, grading, and review always run through the headless CLI regardless of this field; the concrete platform is bound at execution time by the explicit `--cli` flag (SKILL.md), never stored here. A scaffolded runbook ships `"subagent"`, which quality-standards.md § Execution-carrier rules bars from closing a gate — switch to `"cli"` for a gate-closing certification run, the same edit that raises `runs_per_configuration` to 3. The declaration is enforced, not advisory: under `"subagent"`, `run-behavioral` refuses primary-arm configurations and `run-iteration`/`run-runbook` refuse the runbook whole, unless `--override-executor` forces the hermetic run (execution-modes.md)
- `runs_per_configuration`: minimum repetitions per configuration per eval. Also `run-eval`'s (Tier 1) default for `--runs-per-query` when running via `--runbook` and `--runs-per-query` isn't given explicitly — one field dials repetitions for every tier the runbook declares, not just Tier 2/3. An explicit `--runs-per-query` always overrides it; without a runbook (`--eval-set` mode) or when the runbook omits this field, `--runs-per-query` falls back to its own standalone default of 3. `init-runbook` scaffolds this as `1` — the iteration-phase default, cheap enough to re-run after every edit — and any value under 3 stamps `low_n` on the affected summaries. Raise it to at least 3 before the run you gate on: quality-standards.md's small-N rules forbid comparing configurations below that, which is exactly what `gates.require_lift` does
- `gates`: consumed by `grade-result review` — `min_pass_rate` (default 0.8, on the primary configuration's expectation pass rate) and `require_lift` (default true, primary strictly above baseline). A single-configuration benchmark (one config in `run_summary`, hence no `delta` block) resolves that sole configuration as the primary for gating; `require_lift` must then be explicitly `false`, since with no baseline the lift gate can never pass. The shipping bar in quality-standards.md still governs anything the gates don't encode
- `result_root`: where workspaces live, default `result`
- `lang`: optional report/judgment language tag (e.g. `zh-Hant-TW`). When set, `grade-result` adds a language instruction to its grader and reviewer prompts — grading.json's free text (evidence, limitations, eval_feedback, comparison notes) and review.json's assessment/suggestions come back in this language — and `render-report` stamps it as `report.html`'s `<html lang>` attribute. JSON field names, enum values, and each expectation's verbatim `text` stay exactly as written in the suite, and executor prompts are never affected — the language applies to judging and reporting, never to the behavior being measured. Absent = no instruction added anywhere
- `grader`: optional `{model, effort}`, both optional, a declarative default for the model/effort that grades/reviews an iteration (`grade-result run`/`review`'s grader and reviewer dispatch, both — one field for both roles, same as `matrix`'s per-combo `model`/`effort` is one entry for however many roles a combo covers). Each key resolves independently: an explicit `--model`/`--effort` on the invoking command (or `run-runbook`'s `--grader-model`/`--grader-effort`) overrides only that key, same precedence as `--min-pass-rate` overriding `gates.min_pass_rate` above — one key can come from the flag while the other still falls back to this field. A key left unset (or the whole field absent) means no corresponding flag reaches the grader CLI at all (its own default, byte-for-byte the pre-this-field behavior). Recorded provenance either way: `grading.json`'s `grader.model`/`grader.effort`, `review.json`'s `reviewer.model`/`reviewer.effort`. Distinct role from `matrix`'s per-combo `model`/`effort`: quality-standards E11 prefers the grader differ from the executor being judged (same-model grading carries a known leniency bias), so this is deliberately its own field rather than reusing a combo's
  ```yaml
  grader:
    model: claude-sonnet-5
    effort: high
  ```
- `matrix`: optional list of `{model, effort, cli}` combos for Tier 2/3 **execution** (`run-iteration`). This is the runbook's only declarative executor model/effort mechanism — there is no top-level executor `model` or `effort` field (`grader` above is a different role, not an exception to this). A runbook without a `matrix` leaves both to the `run-iteration --model`/`--effort` flags, and with neither given runs on the CLI's own defaults; pin a model declaratively by writing a one-entry matrix. Tier 1 is unaffected either way — trigger measurement stays flag-driven (`run-eval --model`/`--effort`), never matrix-driven. A model id must be valid for the concrete `cli` it runs under, so a runbook intended for both platforms should scope its entries with `cli:` rather than assume one id works everywhere

```yaml
matrix:
  - model: claude-sonnet-5
    effort: high
  - model: claude-sonnet-5
    effort: medium
  - model: claude-haiku-4-5     # effort omitted -> CLI default
  - cli: codex                  # scoped: runs only when the bound cli matches
    model: gpt-5.2
    effort: high
```

**`matrix` validation** (`loadRunbook`, `run-iteration`'s `buildCombos`): each entry must be a mapping using only the keys `model`, `effort`, `cli`; `cli`, when given, must be `"claude"` or `"codex"`; `model`/`effort`, when given, must be strings — any violation raises a load error before any run starts. An entry's `cli:` scopes it to that platform: an entry without `cli:` runs under whichever CLI is bound at execution; one whose `cli:` doesn't match the bound CLI is skipped with a stderr notice, not an error — this is how the `{cli: codex, ...}` entry above only fires from Codex. `model: null` inside an entry is legal (CLI default model), so an effort-only sweep is expressible. Each combo gets a directory slug — `<model-or-default>[-<effort>]` lowercased, every character outside `[a-z0-9]` collapsed to `-`, leading/trailing `-` stripped (`model` null → `"default"`) — used for its `iteration-K/<slug>` workspace, nested inside the iteration (Workspace layout, above). A slug that sanitizes to empty (an all-punctuation `model` string) is a usage error, as are two combos sanitizing to the same slug, and a `--model`/`--effort` CLI flag passed alongside a `matrix` (ambiguous: no single combo left for the flag to override).

`evals[]` and `trigger_evals[]` use the eval-suite field vocabulary below.

## Eval suite fields

The versioned eval suite — the `evals[]` / `trigger_evals[]` sections of a runbook. (A standalone `evals.json` file with the same shape is the portable form for a target that lives outside a runbooks-style repo; the field contract is identical.)

```json
{
  "schema_version": 1,
  "skill_name": "example-skill",
  "evals": [
    {
      "id": 1,
      "eval_name": "descriptive-name",
      "type": "behavioral",
      "priority": "P0",
      "prompt": "User's task prompt",
      "expected_output": "Description of expected result",
      "files": ["runbooks/fixtures/sample1.pdf"],
      "expectations": [
        "The output includes X",
        "The skill used script Y"
      ]
    }
  ],
  "trigger_evals": [
    {"query": "realistic user prompt", "should_trigger": true},
    {"query": "near-miss prompt", "should_trigger": false}
  ]
}
```

**Fields:**
- `schema_version`: `1`. A write-only forward-compat marker — no script parses it. Regression mode (SKILL.md) reads it at load time: missing or unrecognized values mean the suite predates/postdates this contract, so reconcile field names against this doc before grading and warn in the report
- `skill_name`: matches the skill's frontmatter `name`
- `evals[].id`: unique integer
- `evals[].eval_name`: short descriptive slug (doubles as the workspace directory name)
- `evals[].type`: `"behavioral"` (Tier 2) or `"pressure"` (Tier 3)
- `evals[].priority`: optional `"P0"` | `"P1"` | `"P2"`. Absent = current unweighted behavior. Must be copied into `eval_metadata.json` at run time — the aggregator reads eval-level data from the workspace copy, not from this file — so per-priority splits have a data source. Per-priority pass-rate buckets are small-N: report raw counts, not just a rate, and apply `low_n` the same as any other bucket. The shipping bar (quality-standards.md), not this schema, defines the gate: a P0 case the last-known-good version passed must not regress in the candidate
- `evals[].prompt`: the task to execute
- `evals[].expected_output`: human-readable description of success
- `evals[].files`: optional input fixture files, e.g. `runbooks/fixtures/sample1.pdf` — resolved against the current working directory / repo root in BOTH the runbook and the portable `evals.json` forms. (`run-behavioral`'s `copy_fixtures()`/`_validate_fixture_path()` join every entry against `repo_root` uniformly, whichever form supplied it, and reject one that escapes it. An earlier revision of this doc described the portable form as skill-root-relative; that was never how the harness actually resolved it, and the sentence is superseded.)
- `evals[].expectations`: verifiable assertion statements (empty while drafting; required before grading)
- `trigger_evals[]`: Tier 1 query set (see scenario-design.md for design rules)

**Enforced at load, in both carriers.** `evals[]` is validated when the suite is read — by `loadRunbook` for a runbook, and by `run-behavioral`'s eval-set branch for the portable `evals.json` — so a shape error is reported against the file that contains it, naming `evals[N].<field>`, before any run starts. `id` (unique integer), `eval_name`, and `prompt` are required; `type`, `priority`, and `files` are checked when present. `expected_output` and `expectations` are deliberately not required at load (the latter is legal empty "while drafting"), and unknown keys pass through. An absent, null, or empty `evals` is legal — that is the freshly scaffolded shape.

A non-integer `id` is the mistake this guard exists for: `--eval-id` selects a run by that value and parses integers only, so a slug there is unreachable by the very command the eval exists to feed — and before the guard it surfaced as an arg-parser complaint naming the flag, pointing away from the runbook. Duplicate ids are rejected for the same reason: lookup takes the first match, so a second eval sharing an id would silently never run.

## eval_metadata.json

The per-eval, per-workspace copy of an eval-suite entry, written at `iteration-N/<eval-name>/eval_metadata.json` when the eval is dispatched. It freezes the eval's shape for this iteration so later grading and aggregation don't depend on the runbook suite still matching (the suite may have moved on).

```json
{
  "eval_id": 1,
  "eval_name": "ocean-report",
  "type": "behavioral",
  "priority": "P0",
  "prompt": "User's task prompt",
  "expectations": ["The output includes X", "The skill used script Y"],
  "files": ["runbooks/fixtures/sample1.pdf"]
}
```

**Fields:** same meaning as the matching eval-suite fields, with `id` renamed to `eval_id` (this is the load-bearing name every downstream script and the viewer read — do not write `id` here).

**Who reads what:** `aggregate-benchmark` reads `eval_id` and `eval_name` to build `benchmark.json`, and uses the file's existence to identify a directory as an eval dir (vs. a config or run dir) while walking the workspace. The eval-viewer reads `prompt` and `eval_id` for display. `expectations[]` is not parsed by any script — it is the grader's input contract (`agents/grader.md`), read directly by the grader agent.

**Discovery:** the eval-viewer finds the metadata for a given run by searching upward from the run directory to the workspace root and using the nearest `eval_metadata.json` it finds — not a fixed relative path. Keep the file at the eval directory (one level above the configuration directories) so this resolves in one hop.

## grading.json

Output of the grader agent (`agents/grader.md`), one per run directory. The `expectations` array MUST use fields `text`, `passed`, `evidence` — not `name`/`met`/`details` or variants.

```json
{
  "status": "complete",
  "limitations": [],
  "grader": {"cli": "claude", "model": "claude-sonnet-5", "duration_ms": 26000},
  "expectations": [
    {
      "text": "The output includes the name 'John Smith'",
      "passed": true,
      "evidence": "Found in transcript Step 3: 'Extracted names: John Smith, Sarah Johnson'"
    }
  ],
  "summary": {"passed": 2, "failed": 1, "total": 3, "pass_rate": 0.67},
  "execution_metrics": {
    "tool_calls": {"Read": 5, "Write": 2, "Bash": 8},
    "total_tool_calls": 15,
    "total_steps": 6,
    "errors_encountered": 0,
    "output_chars": 12450,
    "transcript_chars": 3200
  },
  "timing": {
    "executor_duration_seconds": 165.0,
    "grader_duration_seconds": 26.0,
    "total_duration_seconds": 191.0
  },
  "claims": [
    {
      "claim": "The form has 12 fillable fields",
      "type": "factual",
      "verified": true,
      "evidence": "Counted 12 fields in field_info.json"
    }
  ],
  "user_notes_summary": {
    "uncertainties": ["Used 2023 data, may be stale"],
    "needs_review": [],
    "workarounds": ["Fell back to text overlay for non-fillable fields"]
  },
  "eval_feedback": {
    "suggestions": [
      {
        "assertion": "The output includes the name 'John Smith'",
        "reason": "A hallucinated document that mentions the name would also pass"
      }
    ],
    "overall": "Assertions check presence but not correctness."
  },
  "expected_output_comparison": {
    "points": [
      {"status": "matched", "note": "Output includes the requested ocean depth chart"},
      {"status": "partial", "note": "Includes the summary table but omits the confidence interval column"}
    ]
  }
}
```

`eval_feedback` is the grader's meta-critique of the eval set itself — present only when it identifies assertions a clearly-wrong output would also pass, or important outcomes no assertion covers.

**`grader`** (optional provenance, written by `grade-result`): `{"cli": "claude"|"codex", "model": "<id or null>", "effort": "<value or null>", "duration_ms": <int>}` — which concrete headless CLI, model, and effort graded this run (resolved per the runbook's `grader` field and any explicit `--model`/`--effort` override — schemas.md § runbook). No script keys off it; it exists so a stricter or drifted judge is distinguishable from a target regression when reading old workspaces (same motivation as `history.json`'s `grader_model`).

**`status`**: `"complete"` (default when absent — legacy grading.json files without the field are treated as complete) or `"inconclusive"`. Use `"inconclusive"` when the HARNESS failed — missing transcript, missing outputs, a crashed executor — not when the target genuinely did poorly. Do not disguise an infrastructure error as a target failure by grading it all-fail: mark it inconclusive instead. `aggregate-benchmark` excludes only runs with an explicit non-complete status; a missing field is never treated as an exclusion (every pre-existing grading.json predates this field). Inconclusive runs are excluded from pass rates and from the primary/baseline delta, and reported as a loud warning rather than silently dropped. `limitations[]` is an optional array of short strings recording what made the run inconclusive or partially gradable (e.g. `"transcript.md missing — grader worked from outputs only"`) — free text, no fixed shape, read by humans reviewing the run.

**`expected_output_comparison`** (optional, additive): present only when `grade-result run` was given `--runbook <path>` (the eval's `expected_output` lives in the runbook, never in `eval_metadata.json` — executor blinding), the matching eval defines a non-empty `expected_output`, AND the grader returned a valid comparison for it — `{points: [{status: "matched"|"partial"|"missing", note: string}]}`, one point per element of the expected output the grader could isolate. Produced by the same single grading call as the rest of this file, not a second grading pass; written as the file's last key, after `grader`. Absent on any eval without an `expected_output`, on any run graded without `--runbook`, and on every `grading.json` written before this field existed — consumers must treat it as optional; `render-report` shows "not compared" for a run where it's absent.

## metrics.json

Written into `<run-dir>/outputs/metrics.json`. On the CLI-driven Tier 2/3 hermetic path this is mechanical, never hand-authored: `run-behavioral`'s `derive_metrics()` computes it from the run's raw event stream after the process exits. Claude: `tool_calls`/`total_steps`/`errors_encountered`/`output_chars` come from the `assistant`/`user`/`result` stream-json events (tool_use blocks; `is_error` tool_result blocks; the last `result` event's text). Codex: the same fields come from `item.completed` events — each non-prose item type counts as a tool call (`agent_message`/`reasoning`/`plan_update`/`todo_list` excluded), a nonzero `command_execution` exit code counts as an error, and the last `agent_message` supplies `output_chars`. `files_created` and `transcript_chars` are computed afterward from the `outputs/` listing and the rendered `transcript.md`, for both CLIs alike. Wherever a subagent executes the run instead (`executor: subagent`, or a no-CLI fallback) there is no event stream to derive from, so the subagent writes it by hand using the same field contract — at `<run-dir>/outputs/metrics.json`, never the run-dir root, where it would be silently ignored.

```json
{
  "tool_calls": {"Read": 5, "Write": 2, "Bash": 8, "Edit": 1, "Glob": 2, "Grep": 0},
  "total_tool_calls": 18,
  "total_steps": 6,
  "files_created": ["filled_form.pdf", "field_values.json"],
  "errors_encountered": 0,
  "output_chars": 12450,
  "transcript_chars": 3200
}
```

## timing.json

Per-run wall clock, token, and cost data, at `<run-dir>/timing.json`. Tier 2/3 hermetic runs (`run-behavioral`) write it mechanically from the driver's event stream (`executors.<cli>.extract_usage()` plus this run's wall-clock duration and provenance) — subagent-executed runs (`executor: subagent`, or a no-CLI fallback) require hand-capture instead.

**How to capture (subagent-executed runs):** when a subagent task completes, its task notification includes `total_tokens` and `duration_ms`. Save them IMMEDIATELY — they are not persisted anywhere else and cannot be recovered later. Process each notification as it arrives.

```json
{
  "total_tokens": 33247,
  "tokens": {"input": 10, "output": 222, "cache_creation": 15479, "cache_read": 17536},
  "total_cost_usd": 0.0338316,
  "duration_api_ms": 4950,
  "model": "claude-sonnet-5",
  "effort": "high",
  "cli": "claude",
  "duration_ms": 14190,
  "total_duration_seconds": 14.2
}
```

**Fields:**
- `total_tokens`: the formula is CLI-scoped, not universal. **claude**: sum of ALL known parts of `tokens{}` below (input + output + cache_creation + cache_read) — a billed-volume proxy, not just input+output. This definition changed on 2026-07-27; a claude workspace written before that date has `total_tokens` = input+output only, and is not directly comparable to a post-change value without re-deriving it from the raw `tokens{}` breakdown (itself new — pre-change files don't have it). **codex**: the CLI's own reported `total_tokens` when numeric, else a fallback sum of input+output ONLY — codex's fallback deliberately excludes `cache_creation`/`cache_read`, unlike claude's four-part sum, even though `tokens.cache_read` may still be populated independently (from `cached_input_tokens`) without being folded into this total
- `tokens`: per-part breakdown, `{input, output, cache_creation, cache_read}`; a part missing from the driver's usage payload stays `null` — never a fabricated `0`
- `total_cost_usd`: USD, from the claude CLI's result event; always `null` on codex (codex reports no cost) — never a fabricated `0`
- `duration_api_ms`: server-side API duration from the claude CLI's result event; always `null` on codex
- `model` / `effort` / `cli`: this run's provenance — the model id (or `null`), the reasoning-effort value passed (or `null`), and the concrete CLI (`"claude"`/`"codex"`)
- `duration_ms` / `total_duration_seconds`: pre-v2 wall-clock fields, unchanged and still written every run — this run's harness-measured wall time, independent of the token/cost fields above

Cross-CLI comparison of any of these fields stays forbidden (the existing CLI-stratum rule) — the richer fields don't change that.

## benchmark.json

Produced by `aggregate-benchmark` at `iteration-N/benchmark.json`.

```json
{
  "metadata": {
    "skill_name": "pdf",
    "skill_path": "/path/to/pdf",
    "cli": "claude",
    "executor_model": "claude-sonnet-5",
    "executor_effort": "high",
    "analyzer_model": "claude-sonnet-5",
    "timestamp": "2026-01-15T10:30:00Z",
    "evals_run": [1, 2, 3],
    "runs_per_configuration": 3,
    "run_counts": {
      "with_skill": {"1": 3, "2": 3, "3": 2},
      "without_skill": {"1": 3, "2": 3, "3": 3}
    },
    "n_inconclusive": 1
  },
  "runs": [
    {
      "eval_id": 1,
      "eval_name": "ocean-report",
      "configuration": "with_skill",
      "run_number": 1,
      "result": {
        "pass_rate": 0.85,
        "passed": 6,
        "failed": 1,
        "total": 7,
        "time_seconds": 42.5,
        "tokens": 3800,
        "cost_usd": 0.0128,
        "tool_calls": 18,
        "errors": 0
      },
      "expectations": [{"text": "...", "passed": true, "evidence": "..."}],
      "notes": ["Fell back to text overlay for non-fillable fields"]
    }
  ],
  "run_summary": {
    "with_skill": {
      "runs": 6,
      "passes": "17/20",
      "pass_rate": {"mean": 0.85, "stddev": 0.05, "min": 0.80, "max": 0.90},
      "time_seconds": {"mean": 45.0, "stddev": 12.0, "min": 32.0, "max": 58.0, "n": 6, "n_missing": 0},
      "tokens": {"mean": 3800, "stddev": 400, "min": 3200, "max": 4100, "n": 5, "n_missing": 1},
      "cost_usd": {"mean": 0.0128, "stddev": 0.0021, "min": 0.0095, "max": 0.0161, "n": 5, "n_missing": 1},
      "output_chars": {"mean": 12033, "stddev": 711, "min": 11000, "max": 13000, "n": 6}
    },
    "without_skill": {
      "runs": 6,
      "passes": "7/20",
      "pass_rate": {"mean": 0.35, "stddev": 0.08, "min": 0.28, "max": 0.45},
      "time_seconds": {"mean": 32.0, "stddev": 8.0, "min": 24.0, "max": 42.0, "n": 6, "n_missing": 0},
      "tokens": {"mean": 2100, "stddev": 300, "min": 1800, "max": 2500, "n": 6, "n_missing": 0},
      "cost_usd": {"mean": 0.0065, "stddev": 0.0009, "min": 0.0051, "max": 0.0079, "n": 6, "n_missing": 0},
      "output_chars": {"mean": 7633, "stddev": 450, "min": 7000, "max": 8200, "n": 6}
    },
    "delta": {
      "primary": "with_skill",
      "baseline": "without_skill",
      "pass_rate": "+0.50",
      "time_seconds": "+13.0",
      "tokens": "+1700",
      "cost_usd": "+0.0063"
    }
  },
  "analysis": {
    "non_discriminating_cases": [
      {
        "case_id": "eval-3-form-fill/assertion-2",
        "expectation": "Output is a PDF file",
        "pattern": "passes 100% in both configurations",
        "evidence": "6/6 with_skill, 6/6 without_skill",
        "interpretation": "Weak assertion — the task itself guarantees a PDF; replace or drop it"
      }
    ],
    "always_fail_cases": [],
    "skill_hurting_cases": [],
    "variance_findings": [
      {
        "case_id": "eval-3-form-fill",
        "finding": "high variance in with_skill",
        "raw": "with_skill: 1/2, 2/2, 0/2",
        "implication": "may be flaky — do not trust a single run's verdict",
        "evidence": "run-1 pass_rate 0.5, run-2 pass_rate 1.0, run-3 pass_rate 0.0"
      }
    ]
  },
  "notes": [
    "Assertion 'Output is a PDF file' passes 100% in both configurations - non-discriminating",
    "Eval 3 shows high variance (50% ± 40%) - may be flaky"
  ]
}
```

**Load-bearing details:** `configuration` (not `config`) with exact strings `with_skill` / `without_skill` (or `new_skill` / `old_skill`); per-run stats nested under `result`. The delta is computed primary-minus-baseline resolved by configuration NAME (`with_skill`/`new_skill` primary, `without_skill`/`old_skill` baseline; other names require explicit `--primary`/`--baseline` flags). `passes` carries raw counts (small-N honesty); a config with fewer than 3 runs gains `"low_n": true` and a markdown warning. `tokens.n_missing` counts runs lacking `total_tokens` — token means never silently mix in `output_chars`, which is its own separate stat. `cost_usd.n_missing` follows the same discipline — it counts runs lacking `total_cost_usd`, which is every run on a codex-executed iteration (codex reports no cost); the `cost_usd` stat block itself is always present (`mean`/`stddev`/`min`/`max`/`n`, `n_missing` added whenever it's nonzero), with `mean: null` rather than a fabricated `$0` when a config's cost is entirely missing.

**`metadata.analyzer_model`**: the model that ran the grader/comparator/analyzer subagents for this iteration — fill it with a real value, not a placeholder. Recorded separately from `executor_model` so a stricter or drifted judge is distinguishable from an actual skill regression; all judging roles share one model within an iteration, so one field suffices; prefer a judging model different from executor_model and disclose when they match (quality-standards E11).

**`metadata.executor_effort`**: the reasoning-effort value passed to the executor CLI for this iteration (`null` when not set — the common case outside a matrix run), recorded next to `executor_model`. Together with `metadata.cli` and the runbook's `executor` it forms the `(cli, model, effort, executor)` quadruple `history.json` uses as its regression-stratum key (see history.json below). `matrix-report`'s per-combo table reads only `executor_model`/`executor_effort` from each combo's `benchmark.json` — not `cli`: a single runbook execution binds one CLI for every combo it runs, already recorded once per combo in `benchmark.json`'s `metadata.cli`, so the cross-combo table has no `cli` column.

**`metadata.cli`**: required concrete platform provenance, `"claude"` or `"codex"`, copied from the explicit `--cli` argument used for the iteration. All configurations within an iteration must use this same value.

**CLI identity, not model naming, defines the platform stratum.** Deltas, pass rates, and trigger rates are only ever compared when their concrete `cli` values match — a with-skill-on-Codex vs baseline-on-Claude comparison measures the platforms, not the skill. Cross-platform evidence lives as separate iterations/entries, each internally paired. Model ids remain CLI-specific, but their spelling is not used to infer the platform.

**`metadata.n_inconclusive`**: total count of runs excluded across the whole iteration because `grading.json` `status` was present and not `"complete"` (harness failures — missing transcript, crashed executor). Mirrors the per-run exclusion already applied to pass rates and the delta; surfaced here so a run of exclusions isn't invisible at the aggregate level. `0` when nothing was excluded.

**`metadata.run_counts`**: `{configuration: {eval_id: count}}` — how many runs actually completed for each (configuration, eval) pair, as opposed to `runs_per_configuration`, which is only the max across all of them and can hide asymmetric completion (e.g. primary finished 3 runs on an eval while baseline finished only 2). When counts are unequal within a configuration across evals, or between primary and baseline on the same eval, the markdown report and CLI summary print an unbalanced-runs warning next to the existing `low_n` warning — pooling unequal per-eval counts into one mean silently reweights evals and skews the delta.

**`analysis`**: structured findings from the benchmark analyzer, one array per pattern category — `non_discriminating_cases` (assertion passes regardless of skill — replace it, don't just flag it), `always_fail_cases` (assertion fails regardless of skill — likely broken or unreachable), `skill_hurting_cases` (fails with the skill, passes without it — the most alarming category), and `variance_findings` (inconsistent verdicts across runs of the same case). Each entry carries `evidence` and, for variance findings, raw per-run counts rather than mean/stddev (mean ± stddev on n < 5 is decoration). Prose duplicates of the same findings may still appear in `notes[]` for human readers — `analysis` is the machine-actionable form eval-integrity tooling can key off, not a replacement.

## comparison.json

Output of blind A/B comparison, at `<grading-dir>/comparison-N.json`. The comparator (`agents/comparator.md`) judges ONE ordering per invocation and is never told which ordering it sees. The caller runs it twice — original order (AB) and swapped (BA) — and combines:

```json
{
  "identity_map": {
    "A": "with_skill",
    "B": "without_skill",
    "output_a": "iteration-2/eval-3-form-fill/with_skill/run-1/outputs/",
    "output_b": "iteration-2/eval-3-form-fill/without_skill/run-1/outputs/"
  },
  "rubric": {
    "dimensions": [
      {"name": "content", "criteria": ["correctness", "completeness", "accuracy"]},
      {"name": "structure", "criteria": ["organization", "formatting", "usability"]}
    ]
  },
  "judgments": [
    {
      "order": "AB",
      "per_dimension": [
        {"dimension": "content", "verdict": "A", "rationale": "..."},
        {"dimension": "structure", "verdict": "tie", "rationale": "..."}
      ],
      "verdict": "A",
      "verdict_as_seen": "A",
      "rationale": "..."
    },
    {
      "order": "BA",
      "per_dimension": [
        {"dimension": "content", "verdict": "A", "rationale": "..."},
        {"dimension": "structure", "verdict": "A", "rationale": "..."}
      ],
      "verdict": "A",
      "verdict_as_seen": "B",
      "rationale": "..."
    }
  ],
  "final_verdict": "A",
  "notes": "Both orderings agreed."
}
```

**Fields:**
- `identity_map`: written by the caller AFTER both comparator invocations complete — the comparator itself never sees or writes it, so it can't leak the blinding. Records which configuration each blind label (`A`/`B`) actually was, plus the output paths judged, so the comparison is auditable after the fact without re-running it. This is the only place the label→skill mapping is persisted; `analysis.json` only implies it indirectly through `winner_skill` and is absent entirely on tie/inconsistent outcomes, so put the mapping here, not there
- `judgments[].order`: `"AB"` (as given) or `"BA"` (labels swapped; the caller un-swaps verdicts before recording)
- `judgments[].verdict`: the un-swapped verdict (what it means for A/B as defined in `identity_map`) — this is what `final_verdict` aggregates
- `judgments[].verdict_as_seen`: the raw verdict the comparator returned for that invocation, before un-swapping — kept alongside `verdict` so the un-swap arithmetic itself is auditable (for the `"BA"` judgment, `verdict_as_seen` is `A` exactly when `verdict` is `B`, and vice versa)
- `verdict` values: `"A"`, `"B"`, `"tie"`, or `"inconclusive"` (the ordering broke the comparison protocol itself — inaccessible output, compromised blindness; `agents/comparator.md` § Inconclusive Verdicts) — categorical, with rationale. Ties are a legitimate outcome; no numeric scores
- `final_verdict`: the agreement of the two judgments; `"inconsistent"` when they disagree, `"inconclusive"` when either ordering was — both get no-evidence handling, but they are distinct facts and `notes` records which applied (SKILL.md § Blind A/B)

## analysis.json

Output of the post-hoc analyst (`agents/analyst.md`), at `<grading-dir>/analysis.json`. Runs unblinded AFTER comparison, converting the verdict into prioritized improvements.

```json
{
  "comparison_summary": {
    "final_verdict": "A",
    "winner_skill": "path/to/winner/skill",
    "loser_skill": "path/to/loser/skill",
    "comparator_reasoning": "Brief summary of the comparator's rationale"
  },
  "winner_strengths": [
    {
      "claim": "Clear step-by-step instructions for multi-page documents",
      "evidence": [
        {"source": "runs/eval-3-form-fill/with_skill/run-1/transcript.md", "quote": "Step 4: split by page count, process each page's fields separately"}
      ],
      "confidence": "high"
    }
  ],
  "loser_weaknesses": [
    {
      "claim": "Vague instruction 'process appropriately' led to inconsistent behavior",
      "evidence": [
        {"source": "runs/eval-3-form-fill/without_skill/run-2/transcript.md", "quote": "unsure how to proceed, tried filling fields directly then fell back to text overlay"}
      ],
      "confidence": "medium"
    }
  ],
  "instruction_following": {
    "winner": {"assessment": "followed", "issues": ["Minor: skipped optional logging step"]},
    "loser": {"assessment": "diverged", "issues": ["Invented own approach instead of following step 3"]}
  },
  "root_causes": [
    {
      "category": "target",
      "finding": "Loser skill's OCR fallback instruction is missing, so the agent gave up on the first failure",
      "affected_cases": ["eval-03"],
      "evidence": "loser_skill/SKILL.md has no fallback section; loser transcript Step 4: 'OCR failed, unable to proceed'",
      "confidence": "high"
    }
  ],
  "improvement_suggestions": [
    {
      "priority": "high",
      "category": "instructions",
      "scope": "skill",
      "suggestion": "Replace 'process the document appropriately' with explicit steps",
      "expected_impact": "Would eliminate the ambiguity that caused inconsistent behavior",
      "evidence": [
        {"source": "runs/eval-3-form-fill/without_skill/run-2/transcript.md", "quote": "unsure how to proceed, tried filling fields directly then fell back to text overlay"}
      ],
      "confidence": "high",
      "validation": "Rerun eval-02 (>=3 reps) with the revised skill in a new iteration; currently-passing evals are the regression guard"
    }
  ],
  "transcript_insights": {
    "winner_execution_pattern": "Read skill -> followed 5-step process -> used validation script",
    "loser_execution_pattern": "Read skill -> unclear on approach -> tried 3 different methods"
  }
}
```

`improvement_suggestions[].category` ∈ instructions | tools | examples | error_handling | structure | references. `priority` = "would fixing this plausibly flip the outcome?" `confidence` ∈ high | medium | low — categorical, never numeric — recording how certain the analyst is that the cited evidence actually caused the outcome (per analyst.md's "consider causation" guideline), which is a separate judgment from `priority`. `evidence[]` entries pair a workspace-relative `source` path with a `quote` locating the exact moment in the transcript or output, matching the grader's citation standard (`grading.json`'s `evidence` field) — a bare path with no quote is not sufficient. `winner_strengths`/`loser_weaknesses` are `{claim, evidence[], confidence}` objects, not bare strings, so every strength/weakness claim is citable the same way root-cause suggestions are. `scope` ∈ skill | harness | eval-suite — which layer the fix belongs to, since not every improvement is a skill-text change. `validation` states the concrete rerun/regression-guard plan that would confirm the fix worked. `root_causes[].category` ∈ target | executor | harness | grader | comparator | environment | unknown — the layer responsible for the observed outcome, distinct from `improvement_suggestions[].scope` (root cause explains *why*; scope says *where to fix it*). `affected_cases` lists eval IDs; `evidence` is a citation string (path + quote-equivalent detail) grounding the finding.

## history.json

The regression ledger, at workspace root. Every completed iteration and every regression re-run appends here — this is the file that answers "did the skill get better or worse over time?" and "does it still hold up on the current model?"

```json
{
  "schema_version": 2,
  "started_at": "2026-01-15T10:30:00Z",
  "skill_name": "pdf",
  "current_best": "v2",
  "iterations": [
    {
      "version": "v0",
      "parent": null,
      "kind": "baseline",
      "cli": "claude",
      "model": "claude-sonnet-5",
      "effort": null,
      "executor": "cli",
      "grader_model": "claude-sonnet-5",
      "timestamp": "2026-01-15T10:30:00Z",
      "expectation_pass_rate": 0.65,
      "raw": "13/20",
      "grading_result": "baseline",
      "is_current_best": false
    },
    {
      "version": "v2",
      "parent": "v1",
      "kind": "improvement",
      "cli": "claude",
      "model": "claude-sonnet-5",
      "effort": "high",
      "executor": "subagent",
      "grader_model": "claude-sonnet-5",
      "timestamp": "2026-01-16T09:00:00Z",
      "expectation_pass_rate": 0.85,
      "raw": "17/20",
      "grading_result": "won",
      "is_current_best": true
    },
    {
      "version": "v2",
      "parent": "v2",
      "kind": "regression-check",
      "cli": "claude",
      "model": "claude-fable-5",
      "effort": "high",
      "executor": "cli",
      "grader_model": "claude-fable-5",
      "timestamp": "2026-06-01T12:00:00Z",
      "expectation_pass_rate": 0.90,
      "raw": "18/20",
      "grading_result": "held",
      "n_inconclusive": 1,
      "is_current_best": true
    }
  ]
}
```

**Fields:**
- `schema_version`: must be `2`. This revision does not accept v1 history ledgers and provides no migration or compatibility path; reconcile old data into the v2 shape before using it for regression decisions
- `kind`: `"baseline"` | `"improvement"` | `"regression-check"`
- `cli`: required concrete platform, `"claude"` or `"codex"`, copied from the explicit `--cli` argument. Together with `model`, `effort`, and `executor` it forms the regression-stratum key: `held`/`regressed` verdicts are computed only against a last-known-good entry with the SAME `(cli, model, effort, executor)` quadruple; that quadruple's first entry is a new baseline, not a comparison across platform, model, effort, or carrier
- `model`: executor model — regression entries across model versions within the same `cli`/`effort` reveal drift, including the retirement signal (baseline now passes → the skill may no longer be needed). Never infer CLI identity from the model id
- `effort`: optional reasoning-effort value passed to the executor CLI for this entry; `null` when not set (the common case outside a matrix runbook, or on a CLI/model that ignores effort). Part of the regression-stratum key — two entries with the same `cli`/`model` but different `effort` are different strata, held/regressed independently rather than averaged together
- `executor`: required carrier, `"cli"` or `"subagent"`, copied from the runbook's `executor` field as executed. The fourth component of the regression-stratum key: a `subagent` entry and a `cli` entry measure different things (the subagent lift is an upper bound — quality-standards.md § Execution-carrier rules), so the two never form each other's baseline
- `grader_model`: the model that ran the grader/comparator/analyzer subagents for this entry. Recorded next to `model` so a stricter or drifted judge is distinguishable from an actual skill regression — a `"regressed"` result where `grader_model` changed but `model` didn't is a grading-drift signal, not necessarily a skill problem
- `grading_result`: `"baseline"`, `"won"`, `"lost"`, `"tie"`, `"inconclusive"`, or `"held"` / `"regressed"` for regression checks vs the last-known-good entry. `"inconclusive"` records that the run(s) behind this entry hit `grading.json` `status: "inconclusive"` (harness failure, not a target result) — inconclusive entries are excluded from pass-rate and delta comparisons against neighboring entries, and reported as a caveat rather than silently folded into "lost"
- `n_inconclusive`: optional, mirrors `benchmark.json`'s `metadata.n_inconclusive` — the count of underlying runs excluded for this iteration because their `grading.json` status was not `"complete"`. Lets an entry with `grading_result` other than `"inconclusive"` still record that some of its runs were harness failures excluded from the pass rate above, rather than silently folding that provenance into the aggregate number. Absent or `0` when nothing was excluded
- Write an entry after EVERY completed iteration or regression run (SKILL.md § Regression mode)

## feedback.json

Human review output, written by the eval-viewer (`eval-skill generate-review`) as a reviewer works through runs in the browser, and read back by `SKILL.md`'s review step.

```json
{
  "status": "in_progress",
  "reviews": [
    {"run_id": "eval-3-form-fill-with_skill-run-1", "feedback": "", "timestamp": "2026-01-16T09:12:00Z"},
    {"run_id": "eval-3-form-fill-without_skill-run-1", "feedback": "Missed the second signature field entirely", "timestamp": "2026-01-16T09:14:00Z"}
  ]
}
```

**Fields:**
- `status`: `"in_progress"` (auto-saved after each review, before submit) or `"complete"` (written on submit, when all runs have been reviewed). Editing feedback after a complete submit resets `status` back to `"in_progress"`
- `reviews[].run_id`: derived by the viewer from the run directory path relative to the served directory, with path separators replaced by `-` — e.g. serving `<workspace>/iteration-2` and reviewing `eval-3-form-fill/with_skill/run-1` yields `eval-3-form-fill-with_skill-run-1`
- `reviews[].feedback`: free text. Empty string is a valid, meaningful value — it means the reviewer looked at this run and it was fine, not that the run was skipped. On a complete submit, `reviews[]` includes every run that was served, whether or not feedback was entered
- **Location:** written to `<workspace>/iteration-N/feedback.json` (the served directory), per-iteration, never the workspace root — matches the layout diagram above

## review.json

Output of `grade-result review`, at `iteration-N/review.json` (with a human-readable twin `review.md`). The gate verdict is computed mechanically from `benchmark.json`; the headless reviewer process contributes only `assessment` and `suggestions`.

```json
{
  "schema_version": 2,
  "skill_name": "example-skill",
  "verdict": "fail",
  "gates": [
    {"name": "min_pass_rate", "threshold": 0.8, "observed": 0.72, "raw": "13/18", "passed": false},
    {"name": "require_lift", "threshold": "> 0 vs baseline", "observed": "+0.30", "passed": true},
    {"name": "no_unrepaired_harness_failures", "threshold": 0, "observed": 0, "passed": true}
  ],
  "assessment": "Short diagnosis of why the iteration landed where it did.",
  "suggestions": [
    {
      "priority": "high",
      "scope": "skill",
      "suggestion": "Replace 'process the document appropriately' with explicit steps",
      "evidence": "eval-3 run-2 failed 'output lists every field' — transcript shows the executor guessing",
      "expected_impact": "eval-3 expectation flips to pass",
      "validation": "Rerun eval-3 (>=3 reps) in a new frozen iteration"
    }
  ],
  "reviewer": {"cli": "claude", "model": null, "duration_ms": 48210, "error": null}
}
```

**Fields:**
- `schema_version`: `2`. Review v2 records concrete CLI provenance as `reviewer.cli`; the former platform-selector field name is unsupported
- `verdict`: `"pass"` | `"fail"` | `"inconclusive"` — mechanical, from `gates[]`; `inconclusive` when the benchmark lacks a usable primary configuration. The model never decides this
- `gates[]`: one entry per checked gate with the observed value (and raw counts where available — small-N honesty applies here too)
- `suggestions[]`: improvement HYPOTHESES with `scope` ∈ skill | eval-suite | harness — same layer vocabulary as `analysis.json`'s `improvement_suggestions[].scope`; validate in a new frozen iteration, never by re-reading the same runs
- `reviewer`: provenance of the headless CLI process; `cli` is the concrete value passed to the review command; `model`/`effort` resolve the same way as `grading.json`'s `grader` fields above; `error` non-null means the reviewer failed (verdict still stands, suggestions absent)

`render-report` consumes `benchmark.json`, this file, per-run `timing.json`/`grading.json`, `trigger_results.json`, and the runbook itself to produce the runbook's `report.html` (Workspace layout, above). `grade-result review` invokes it automatically after writing this file (`--no-render` opts out), so review.json and report.html normally appear together.

## trigger_results.json

Written by `run-eval --out PATH` (Tier 1), at `<result_root>/<runbook>/iteration-N/trigger_results.json` — per-iteration, alongside that iteration's `matrix.md`/`report.html` (see Workspace layout above). Read by `render-report` to label the report's trigger-results section with what actually ran. `--out` is additive: `run-eval`'s stdout contract (the same JSON `formatEvalOutputJson` prints) is unchanged — the file adds exactly one extra top-level field the stdout never carries.

```json
{
  "invocation": {"cli": "claude", "model": "claude-sonnet-5", "effort": "high"},
  "skill_name": "example-skill",
  "description": "This skill should be used when...",
  "cli": "claude",
  "results": [
    {
      "query": "realistic user prompt",
      "should_trigger": true,
      "trigger_rate": 0.67,
      "triggers": 2,
      "runs": 3,
      "errors": 0,
      "pass": true
    }
  ],
  "summary": {"total": 20, "passed": 18, "failed": 1, "inconclusive": 1},
  "effort": "high",
  "runbook": "runbooks/example-skill.yaml"
}
```

**Fields:**
- `invocation`: file-only wrapper added on top of the unchanged stdout object — never present in `run-eval`'s stdout. `cli`/`model`/`effort` are the concrete platform/model/effort this Tier 1 run used; `model` and `effort` are `null` when the corresponding flag wasn't passed. `invocation.cli`/`invocation.effort` duplicate the stdout's own top-level `cli`/`effort` fields (below) into one self-contained provenance block; `invocation.model` is new information the stdout never carries at all
- `skill_name` / `description` / `cli` / `results[]` / `summary` / `effort` / `runbook`: the unchanged `run-eval` stdout report — `results[]` holds one entry per trigger query (`query`, `should_trigger`, `trigger_rate`, `triggers`, `runs`, `errors`, `pass`), with `trigger_rate`/`pass` staying `null` when every run of that query errored (that query counts toward `summary.inconclusive`, not `passed`/`failed`); `summary` totals `total`/`passed`/`failed`/`inconclusive` across all queries; `runbook` is present only when `run-eval` ran in `--runbook` mode (literally absent, not `null`, in `--eval-set` mode) — which is how the runbook choreography always invokes it
