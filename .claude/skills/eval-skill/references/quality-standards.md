# Skill Quality Standards

The single normative rulebook for skill quality. create-skill authors against these rules; eval-skill checks them. Each rule has an ID, a threshold, and a check method:

- **lint** — enforced automatically by `eval-skill validate-skill` (Tier 0)
- **rubric** — judged by a grader/reviewer agent against evidence
- **human** — needs human judgment (surfaced in review, never auto-failed)

Information lives HERE, not duplicated in SKILL.md files. When a rule needs teaching (examples, rationale), that lives in create-skill's `references/writing-guide.md`, which cites rule IDs from this file.

## Structure rules

| ID | Rule | Check |
|----|------|-------|
| S1 | `SKILL.md` exists with YAML frontmatter that parses | lint |
| S2 | Frontmatter has `name` and `description`; other keys limited to `license`, `allowed-tools`, `metadata` | lint |
| S3 | `name` matches `^[a-z0-9]+(-[a-z0-9]+)*$`, ≤ 64 chars, and equals the directory name | lint |
| S4 | Every relative file path referenced in SKILL.md exists in the skill directory | lint |
| S5 | Reference files sit at most one directory level below the skill root | lint (warn) |
| S6 | No forbidden auxiliary files: README.md, INSTALLATION_GUIDE.md, QUICK_REFERENCE.md, CHANGELOG.md, or similar meta-documentation | lint |
| S6a | Root-level documents besides `SKILL.md` (any file with a document suffix that isn't on the S6 forbidden list, e.g. a stray `.md`/`.txt` at the skill root) are WARN, not FAIL: fold the essential details into `SKILL.md` or `references/` (`root.extraneous_doc`) | lint (warn) |
| S7 | Every bundled file is referenced from SKILL.md (so the agent knows it exists and when to read it) | lint (warn) |
| S8 | No leftover `[TODO` scaffold markers anywhere in SKILL.md | lint |

## Size budgets

| ID | Rule | Check |
|----|------|-------|
| B1 | SKILL.md body ≤ 500 lines and < 5,000 words (hard ceiling) | lint |
| B2 | Working target 1,500–2,000 words; approaching B1 means push detail into `references/`, not amputate content | rubric |
| B3 | Metadata (name + description) ~100 words — it is always in context for every conversation | rubric |
| B4 | Reference files > 100 lines include a table of contents | rubric |
| B5 | Reference files > 10k words get grep search patterns listed in SKILL.md | rubric |
| B6 | No duplication: information lives in SKILL.md or a reference file, never both | rubric |

Progressive disclosure is the size-control mechanism: metadata always loaded → body loaded on trigger → bundled resources loaded (or executed without loading) as needed.

## Description rules

| ID | Rule | Check |
|----|------|-------|
| D1 | ≤ 1024 chars, no angle brackets | lint |
| D2 | Third person ("This skill should be used when…" / "Use when…"), never first or second person | lint (heuristic) + rubric |
| D3 | States capability at intent level AND concrete triggers: user phrases, symptoms, file types, contexts | rubric |
| D4 | NEVER summarizes the skill's process or workflow. A description that summarizes the steps becomes a shortcut the agent follows instead of reading the body | lint (heuristic: numbered-step patterns) + rubric |
| D5 | All "when to use" information lives in the description — the body loads only after triggering, so a "When to Use" body section can't influence triggering | rubric |
| D6 | Keyword coverage: error messages, symptoms, synonyms, tool names an agent would match on | rubric |
| D7 | No artificial "pushiness" ("use this even if the user doesn't ask"). Trigger quality is measured with trigger evals, not inflated with prose | rubric |
| D8 | Technology-agnostic triggers unless the skill is technology-specific — then the technology is named explicitly | rubric |

## Body style rules

| ID | Rule | Check |
|----|------|-------|
| W1 | Imperative/infinitive form ("Run the script", not "You should run…") | rubric |
| W2 | Explain the why behind non-obvious instructions instead of bare ALL-CAPS MUSTs. ALL-CAPS ALWAYS/NEVER is a yellow flag: reframe with reasoning | rubric |
| W3 | One excellent, complete, realistic example beats many mediocre ones. No multi-language dilution | rubric |
| W4 | No narrative storytelling ("In session X we found…") — skills are reference guides, not incident reports | rubric |
| W5 | Cross-reference other skills by name with requirement markers (REQUIRED BACKGROUND / REQUIRED SUB-SKILL). Never `@`-link files — `@` force-loads content immediately and burns context | rubric |
| W6 | Flowcharts (mermaid) only for genuinely non-obvious decision points; never for linear steps, reference material, or code | rubric |
| W7 | Specificity matches fragility (degrees of freedom): heuristic text for open decisions, parameterized scripts for preferred patterns, locked scripts for fragile operations | rubric |

## Bundled resource rules

| ID | Rule | Check |
|----|------|-------|
| R1 | Scripts are tested by actually running them before shipping: happy path, one invalid-input case, and one representative failure path (a representative sample when many scripts are similar) | rubric (evidence of execution) |
| R2 | Scripts solve, don't punt: no placeholder implementations, no unexplained magic constants | rubric |
| R3 | A resource earns its place by eliminating repeated work — the signal is test-run agents independently rewriting the same helper | rubric |
| R4 | `scripts/` = executable code (can run without entering context); `references/` = docs loaded on demand; `assets/` = files used in output, never loaded | rubric |
| R5 | Skill contains no malware, exploit code, or content that would surprise the user given the skill's stated intent | human |

## Evaluation rules (what "evaluated" means)

| ID | Rule | Check |
|----|------|-------|
| E1 | The skill has a versioned eval suite in the repo's runbook (`runbooks/<skill-name>.yaml` — or a portable standalone `evals.json` outside a runbooks-style repo), re-runnable for regression; the suite never lives inside the skill directory | rubric (suite presence) |
| E2 | A baseline run failed before the skill was written (behavioral changes only; mechanical edits re-run the existing suite) | rubric (workspace evidence) |
| E3 | An eval the baseline already passes is non-discriminating: it proves the skill unnecessary or the eval broken — EXCEPT forbidden-behavior guard assertions and overcompliance scenarios, which are expected to pass in both configurations by design (they gate, they don't count as lift evidence) | rubric |
| E4 | When an agent fails a skill-owned failure (Instruction/Discovery/Navigation/Resource — see scenario-design.md's failure classification), fix the skill — never weaken the eval. Classify first: Evaluator-class failures are fixed in the runbook suite (noted in the ledger); Runtime-class failures are marked inconclusive and rerun; Variance-class needs more reps, not a fix; Contract-class escalates to the user | rubric |
| E5 | Discipline-enforcing skills passed pressure scenarios (3+ combined pressures) before shipping, AND no overcompliance scenario blocked a legitimate exception or unrelated work | rubric |
| E6 | For each P0/critical-rule assertion, a mutation-control check (disposable mutated copy with the rule removed/inverted) confirms the assertion actually fails — ≥3 mutant reps required to certify; a single run may only flag doubt, never certify. Recorded as workspace evidence, never inside the runbook suite | rubric (workspace evidence) |
| E7 | Eval-suite assertions and expectations are written before inspecting any candidate output; assertions revised after seeing outputs are re-validated against a fresh run before they count. Does not apply to the blind comparator's own post-hoc rubric | rubric |
| E8 | Executors receive only the eval prompt and fixtures — never the assertions, expected answer, desired winner, or which arm of a comparison they're running | rubric |
| E9 | Suite edits (the runbook's `evals[]`) and baseline edits land as their own iteration — never edit skill + suite + baseline in the same measured run | rubric |
| E10 | The grader is validated with a known-good and a known-bad fixture before its verdicts on a new suite are trusted | rubric (workspace evidence) |
| E11 | Prefer a grader/reviewer model different from the executor model (same CLI stratum; pass `--model` to `grade-result`). When they are the same, the report must disclose it — same-model grading carries a known leniency bias. Either way the concrete grader model is recorded (`grading.json` `grader.model`, `history.json` `grader_model`) | rubric |
| E12 | A gate-closing run includes held-out evidence the skill was not tuned to: at least one eval never used as feedback during the improvement iterations that produced the candidate (a held-out split, or a freshly written case pre-registered per E7). A fixed suite iterated against for rounds is a training set — passing it stops evidencing generalization, and the overfit shows up as a BETTER delta, which E3 cannot catch. `grade-result review` runs a mechanical n-gram overlap check between the skill text and `evals[]` prompts (report-only: `review.json` `overfit_check`, flagged evals listed in review.md); a flag must be resolved — generalize the skill wording, or justify it in the ledger — before the gate closes | rubric + mechanical flag |

## Shipping bar (defaults — a skill ships when all applicable gates pass)

The bar is read at two evidence levels, and every report states which one it delivers:

- **Validation (first measured run)** — what executing the suite on a scaffolded runbook's own dials (`executor: subagent`, `runs_per_configuration: 1`) delivers: the gates below evaluated on whatever dials the iteration actually ran, reported with raw counts, the carrier, and `low_n` labels. Honest directional evidence that the skill teaches what it claims. The create-skill authoring loop deliberately hands back *below* this level — hermetic RED baseline plus informal subagent spot-checks, no measured iteration — so the first validation-level run is typically user-initiated, after authoring.
- **Certification (gate-closing)** — required wherever a claim rests on the numbers: a regression verdict, a publish-grade "proven better", a repo's gating suite. Only here do the carrier and small-N rules below bind as gates: `executor: cli`, `runs_per_configuration` ≥ 3, held-out evidence (E12). A validation-level pass is never presented as a closed gate.

| Gate | Default threshold |
|------|-------------------|
| Tier 0 lint | zero FAILs |
| Tier 1 triggering | should-trigger rate ≥ 2/3 per query; near-miss false-trigger rate ≤ 1/3 |
| Tier 2 lift | with-skill assertion pass rate ≥ 80% AND strictly better than baseline on the discriminating assertions AND zero P0 assertion failures (any rep failing a P0 assertion fails the gate) |
| Tier 3 pressure (discipline skills) | every pressure scenario held (undercompliance) AND no overcompliance scenario blocked a valid exception; no new rationalizations in the final iteration |

Small-N honesty rules, applying to every tier:

- Report raw counts ("4/6"), not just means. Mean ± stddev on n < 5 is decoration, not evidence.
- Minimum 3 runs per configuration before comparing configurations.
- A delta within noise of the threshold is "no evidence" — run more, don't round up.
- Never present a single-run pass as validation of anything.
- Iterating at n=1 is not a violation of the rule above, and is what a scaffolded runbook ships with: one run per configuration is a cheap directional signal for "did that edit help", and `low_n` labels it honestly. What n=1 may never do is *close* a gate. Raise `runs_per_configuration` to at least 3 for a gate-closing certification run — diagnosis is cheap, certification is not.
- A run whose failure traces to harness/environment causes (see scenario-design.md's inconclusive verdicts) is neither a pass nor a fail — exclude it from pass-rate math and low-N counts rather than forcing it into one bucket.

## Reporting requirements

The human-facing report states:

- target + baseline versions; tiers run and depth;
- raw counts per configuration, with low-N flags;
- the carrier (`executor`) alongside the CLI and model — a reader cannot infer isolation from a pass rate;
- the delta vs baseline including its time/token/cost, with an explicit judgment of whether the lift justifies that cost — a +50pp lift for +13s and $0.12 is a different decision than +2pp for doubled tokens. Cost is USD, reported to 4 decimal places, and joins the judgment only for claude-executed runs; codex reports tokens only, never a fabricated cost;
- failures grouped by root cause, with cited evidence;
- eval_feedback and non-discriminating findings;
- revision hypotheses labeled explicitly as hypotheses, not conclusions — validated only in a new frozen iteration, never by re-reading the same runs harder;
- limitations, plus the next most valuable eval to add.

Under `executor: cli`, a with_skill sandbox activates the CLI's own skills subsystem (its built-in skills list is present) while an empty baseline sandbox has no skills block at all, so every token/cost delta includes that platform overhead alongside the skill's own — report the measured delta as an upper bound on the skill's cost, not an isolated measurement of it. Under `executor: subagent` the same caveat applies to the *lift itself*, far more strongly (see below).

## Execution-carrier rules

The twin of the small-N rules above, because a scaffolded runbook ships loose on both dials at once (`executor: subagent`, `runs_per_configuration: 1`) and a gate-closing certification run tightens both in the same edit:

- Running with `executor: subagent` is not a violation, and is what a scaffolded runbook ships with: evals run in-harness instead of spawning a hermetic CLI per run, which is a cheap directional signal for "did that edit help". **What subagent execution may never do is *close* a gate.** Switch `executor` to `cli` for a gate-closing certification run — same moment you raise `runs_per_configuration` to 3.
- Two things the subagent carrier structurally cannot deliver — **a clean paired delta** (the lift is an upper bound: the baseline arm is hermetic precisely because a subagent baseline would inherit the session's skill registry) and **`events.jsonl`** (every `check-trace` process assertion silently falls back to LLM judgment) — must both be stated in any report built on it. Mechanics: `execution-modes.md`.
- Carriers are separate result strata, exactly like CLIs. A subagent-executed iteration is never pooled with, nor used as the regression baseline for, a `cli`-executed one; `history.json`'s comparison key carries `executor` alongside `cli`/`model`/`effort` for that reason.
