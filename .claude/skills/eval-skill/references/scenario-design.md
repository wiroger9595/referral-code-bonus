# Eval Scenario Design

How to write scenarios that actually discriminate. Used by eval-skill when authoring standalone or regression evals, and by create-skill when seeding evals from the concrete examples gathered during intent capture.

## Contents

- [Test type by skill kind](#test-type-by-skill-kind)
- [Realistic queries (all scenario types)](#realistic-queries-all-scenario-types)
- [Trigger scenarios (Tier 1)](#trigger-scenarios-tier-1)
- [Behavioral scenarios (Tier 2)](#behavioral-scenarios-tier-2)
- [Resource scenarios (bundled scripts)](#resource-scenarios-bundled-scripts)
- [Eval integrity: pre-registration, blinding, mutation control](#eval-integrity-pre-registration-blinding-mutation-control)
- [Pressure scenarios (Tier 3)](#pressure-scenarios-tier-3)
- [Meta-testing: diagnosing a failed GREEN](#meta-testing-diagnosing-a-failed-green)
- [Classify the failure before fixing anything](#classify-the-failure-before-fixing-anything)
- [Inconclusive verdicts and stop conditions](#inconclusive-verdicts-and-stop-conditions)
- [Micro-testing wording variants](#micro-testing-wording-variants)
- [Rationalization capture format](#rationalization-capture-format)

## Test type by skill kind

Test type varies; testing itself is not optional. "Subjective" exempts a skill from assertions, never from evaluation (it gets human review and blind A/B instead).

| Skill kind | Test with | Success criterion |
|------------|-----------|-------------------|
| **Discipline** (rules with compliance costs: TDD, verification gates) | Pressure scenarios (3+ combined pressures), rationalization capture, overcompliance scenarios | Agent follows the rule under maximum pressure, cites the skill, AND doesn't block legitimate exceptions or unrelated work |
| **Technique** (how-to with steps) | Application scenarios, variation/edge-case scenarios, missing-information probes | Agent applies the technique correctly to a NEW scenario |
| **Pattern** (mental model) | Recognition scenarios, application scenarios, counter-examples | Agent knows when the pattern applies AND when it doesn't |
| **Reference** (API docs, schemas) | Retrieval scenarios, application scenarios, gap testing | Agent finds and correctly applies the right information |
| **Tool** (bundled scripts/APIs are the contract) | Resource scenarios (see below) plus application scenarios | Script runs correctly on real input AND fails safely on bad input; agent uses the bundled script rather than reimplementing it |

## Realistic queries (all scenario types)

Queries must read like something a real user would type. Abstract prompts test nothing.

- Concrete and specific: file paths, column names, company names, URLs, a little backstory.
- Mixed register: some lowercase, typos, abbreviations, casual speech. Mixed lengths.
- Substantive enough that an agent would benefit from consulting a skill — trivial one-step queries ("read this file") don't trigger skills regardless of description quality, so they measure nothing.

Bad: `"Format this data"` · `"Extract text from PDF"`

Good: `"ok so my boss sent me this xlsx (in my downloads, 'Q4 sales final FINAL v2.xlsx') and wants a column showing profit margin as a percentage. revenue is col C, costs col D i think"`

## Trigger scenarios (Tier 1)

A trigger eval set is 16–20 queries, half should-trigger, half should-not:

**Should-trigger (8–10):** cover phrasing diversity — formal and casual versions of the same intent, at least one explicit invocation (the user names the skill outright — "use
eval-skill to …"); a name/description regression breaks these first, cases where the user never names the skill or file type but clearly needs it, uncommon use cases, and cases where this skill competes with a sibling skill but should win.

**Should-not-trigger (8–10):** the only valuable negatives are **near-misses** — queries sharing keywords or concepts with the skill that actually need something else. Adjacent domains; ambiguous phrasing where naive keyword matching would fire; contexts where another tool is more appropriate. "Write a fibonacci function" as a negative for a PDF skill is too easy — it tests nothing.

Cross-skill interference: when sibling skills are installed, also run THEIR should-trigger queries against the new skill's description — a new description that steals a sibling's traffic is a regression even if its own numbers look good.

Triggering is stochastic: run each query ≥ 3 times and score trigger RATE, reported as precision/recall. False triggers count as failures equal to missed triggers.

**What counts as a trigger.** A run is a PASS only if the trial transcript shows an actual Skill invocation or a Read of the skill's SKILL.md — not merely the agent acting consistently with the description. An agent that guesses correct behavior from the description alone, without reading the body, is a **metadata-shortcut** failure (cross-reference D4: it means the description leaked process, not that triggering worked) — record it as its own failure mode, distinct from a plain miss.

**Sibling routing (near-misses).** When the skill has siblings whose descriptions overlap, near-miss queries should assert not just `should_trigger: false` but WHICH sibling should win — add an `expected_selection` field per near-miss query and a small distractor catalog (target + siblings + 1-2 unrelated skills) when siblings exist. This turns "not this skill" into a routing assertion instead of a bare negative.

**Growing the set.** Trigger runs are cheap relative to Tier 2, so volume
helps: generate additional query variants with an LLM from the existing
seeds (vary register, domain nouns, length, typos), then curate with the
user via `assets/eval_review.html` before any variant enters the suite —
uncurated generated queries encode the generator's biases, not the users'.

## Behavioral scenarios (Tier 2)

Each scenario is a task prompt plus assertions (see `schemas.md` → Eval suite fields):

- Assertions are objective, descriptively named, and individually checkable. Programmatically checkable assertions get a script, not eyeballing (command-pattern process assertions: eval-skill's `eval-skill check-trace`).
- Discriminating: an assertion that passes with AND without the skill measures nothing (quality-standards E3). An assertion a clearly-wrong output would also pass is worse than none — it manufactures false confidence.
- Fixtures mirror reality completely. Don't hand the agent a minimal input crafted so the assertions happen to pass — incomplete fixtures are the eval-side version of testing a mock.
- Cover outcomes, not steps: assert what the output IS, not which internal path produced it, unless process compliance is the point of the skill.

**Priority and forbidden behaviors.** Every eval gets a `priority` (P0/P1/P2, default P1 for older suites without the field). A P0 assertion must pass in EVERY rep — one failing rep fails the case, regardless of aggregate pass rate (a critical failure can't be averaged away by cosmetic passes). Assertions may also be negative: a `forbidden` list of behaviors that must NOT occur, graded pass/fail with cited evidence exactly like a positive assertion, and counted in the same denominator. Forbidden-behavior (guard) assertions are expected to pass in BOTH configurations by construction — a safety floor exempt from E3's non-discrimination rule, never lift evidence (quality-standards.md E3). A standing forbidden-behavior example worth including for file-producing skills: the run writes nothing outside its own `outputs/` directory — stray scratch files left in the working directory count as a violation.

## Resource scenarios (bundled scripts)

Scoped to skills where the bundled script/API IS the contract (Tool-kind skills, or any skill where a resource's behavior is what's being shipped) — not a blanket requirement for every skill (see "outcomes, not steps" above). For each bundled executable resource, run direct deterministic tests (no agent in the loop) covering:

- **Happy path** — script runs on valid representative input and produces the expected artifact.
- **Invalid input** — a malformed or out-of-range input is rejected, not silently miscomputed.
- **A representative failure path** — an expected-failure case (e.g. bad config) fails cleanly and leaves no partial state; don't mistake an expected non-zero exit for a harness failure.
- **Determinism** — run twice; outputs are byte-identical after excluding declared nondeterministic fields (timestamps, run IDs). Scripts with no such fields should be timestamp-free.
- **Escaping/injection** where output embeds user data (e.g. a payload like `</script><script>...`) — the script must not let injected content execute or corrupt structure.

Run each test in an isolated workspace (own HOME/TMPDIR, not the skill's dev checkout), and assert the target skill tree is unchanged after the run. This is deterministic, absolute (no baseline needed, like Tier 0) — grade pass/fail with the command's exit code and output as cited evidence. Separately, whether the agent used the bundled script vs. reimplemented it ad hoc is a context-waste / redundant-work signal for the analyzer, not a per-run graded assertion.

## Eval integrity: pre-registration, blinding, mutation control

**Pre-registration.** Write assertions and expected outcomes before inspecting any candidate output. An assertion tailored to a property an inspected with-skill run happens to have will discriminate and pass E3's non-discriminating check while still being fraudulent evidence — it inflates the delta without proving anything general. Assertions authored or revised after seeing outputs (including via the grader's `eval_feedback` meta-critique) must be re-validated against a FRESH run before they count. This applies to eval-suite assertions and expectations; it does not apply to the blind comparator's own task-adapted rubric, which is deliberately generated after reading both outputs because the judge is blind to identity and doubled via AB/BA.

**Executor blinding.** Executors receive only the eval prompt and fixtures — never the assertions, the expected answer, the desired winner, or any framing revealing which arm of a comparison they're running.

**Mutation control.** Before freezing a P0/critical-rule assertion into a regression suite (or when the analyzer flags doubt about whether an assertion is causally tied to the rule it claims to guard), run it against a disposable MUTATED COPY of the skill with only that rule removed or inverted — never adjust the eval based on mutant behavior. A single mutant run is a smoke check only: if the assertion still passes on the mutant, that's a red flag, but it can never certify the assertion (small-N honesty bans single-run validation). Certifying requires ≥3 mutant reps, all failing. Record the result as workspace evidence (quality-standards.md E6), not inside the runbook suite — mutation provenance goes stale the moment the skill is revised, and baking a stale "verified" flag into the shipped suite manufactures false confidence.

**Known-good grader fixture.** Before trusting a new suite's grading, feed the grader one transcript known to pass and one known to fail; confirm its categorical verdicts match. This is a harness self-check, not a new tool — the grader's existing cited-evidence requirement covers most of the mechanics.

## Pressure scenarios (Tier 3)

For discipline skills only. Goal: confirm the agent follows the rule when it WANTS to break it.

**Pressure types — combine 3 or more:**

| Pressure | Example |
|----------|---------|
| Time | Emergency, deadline, deploy window closing |
| Sunk cost | Hours of work it feels wasteful to delete |
| Authority | Senior engineer or manager says skip it |
| Economic | Job, promotion, company survival at stake |
| Exhaustion | End of day, tired, wants to go home |
| Social | Looking dogmatic, seeming inflexible |
| Pragmatic | "Being pragmatic, not dogmatic" |
| One-off | "Just this once, it's temporary" |
| Scope minimization | "This change is too small to need the process" |
| Plausible ambiguity | Genuinely unclear whether the rule applies to this case |

**Scenario requirements:**

1. Forced concrete choice (A/B/C) remains the primary gate for pressure scenarios, plus at least one open-ended task-execution scenario per discipline skill where the tempting shortcut is available but no options are enumerated (an all-A/B/C suite telegraphs that the agent is being tested). The open-ended scenario needs explicit assertions naming the observable evidence of shortcut-taken vs. rule-followed.
2. Real constraints — specific times, actual consequences
3. Real file paths — `/tmp/payment-system`, not "a project"
4. Make the agent act: "Choose and act", not "What should one do?"
5. No easy outs — deferring to "I'd ask my human partner" doesn't count as a choice; this extends to the open-ended scenario too

**Plausible-ambiguity scenarios** must declare which side of the boundary is correct — the "expected_option" (or expected behavior, for open-ended scenarios) is otherwise undefined. Pair each with an under/overcompliance check (see below): does the agent correctly decide the rule applies, and would it also correctly decide the rule doesn't apply in the sibling case?

**Overcompliance scenarios.** Pressure testing so far is one-directional (does the rule hold?). Add 1-2 scenarios per discipline skill where the rule legitimately does NOT apply — an out-of-scope task, or a stated valid exception — and the correct answer is to NOT invoke the rule. A discipline skill that blocks legitimate exceptions or unrelated work fails just as badly as one that folds under pressure; the rationalization-capture loop otherwise hardens skill wording with no counterweight, breeding dogmatic refusal. These scenarios are baseline-passing by construction (the rule shouldn't fire) — the same E3 exemption as forbidden-behavior guards; they gate Tier 3 as those gate Tier 2 (quality-standards.md E3, E5). Mark each scenario with a polarity field (`expected_behavior: comply | exempt`) so the grader doesn't mistake "correctly took the exception" for a violation.

**Setup preamble:**

```
IMPORTANT: This is a real scenario. You must choose and act.
Don't ask hypothetical questions — make the actual decision.
```

An academic prompt ("What does the skill say about X?") only tests recitation. The agent must believe it is doing real work.

**Bulletproof criteria:** the agent picks the correct option under maximum pressure, cites skill sections as justification, and acknowledges the temptation while refusing it. NOT bulletproof: new rationalizations, arguing the skill is wrong, inventing "hybrid approaches", or asking permission while lobbying to violate.

## Meta-testing: diagnosing a failed GREEN

When an agent read the skill and still chose wrong, ask it:

> "You read the skill and chose Option C anyway. How could the skill have been written to make it crystal clear that A was the only acceptable answer?"

Three response types map to three fixes:

1. **"The skill WAS clear; I chose to ignore it"** → not a documentation problem; add a foundational principle ("Violating the letter of the rules is violating the spirit of the rules")
2. **"The skill should have said X"** → wording problem; add their suggestion, often verbatim
3. **"I didn't see section Y"** → organization problem; move the key point earlier and make it prominent

## Classify the failure before fixing anything

Before reacting to any failing run, classify WHY it failed and route the fix to the component that owns it. quality-standards.md E4 ("never weaken the eval") applies only to the skill-owned classes — treating an Evaluator- or Runtime-class failure as an instruction to patch the skill is over-applying E4.

| Class | Typical evidence | Owner / fix |
|-------|-------------------|--------------|
| Contract | Requirements or expected behavior are themselves unclear or contested | Escalate to the user — not a skill or eval bug |
| Discovery | Wrong or no skill selected | Skill description (D-rules) |
| Instruction | Skill loaded, agent still chose wrong | Skill body wording/organization (meta-testing below) |
| Navigation | Right skill, agent missed a reference file or bundled resource it needed | Skill structure (S5, S7, W5) |
| Resource | Bundled script/reference is broken, missing, or misused | Skill's bundled resource |
| Runtime | Missing tool, auth, network, or other environment failure | Not the skill — mark the run **inconclusive**, rerun |
| Evaluator | Fixture, assertion, or grader is broken or non-discriminating | Fix the runbook's eval suite, note the change in the ledger — never touch the skill for this |
| Variance | Failure disappears on rerun; within normal noise | More reps (small-N rules) — not a fix |
| Regression | A run that used to pass now fails after a skill/suite/baseline edit | Isolate which edit caused it (see one-variable rule, quality-standards.md E9) |

Do not rewrite a valid case to conceal a regression — the classification exists to route fixes correctly, not to make failures disappear. The paired with-skill/baseline signal (a case failing in BOTH configurations) is the primary evidence for Evaluator/Resource/Runtime; meta-testing (below) is the Instruction-class sub-classifier.

## Inconclusive verdicts and stop conditions

A run or suite is **inconclusive** — not pass, not fail — when the cause is a harness/environment/evidence-availability problem, never a behavioral judgment call:

- Harness error (crash, missing tool, lost transcript) prevented gradeable output
- Required infrastructure (network, auth, a live service) was unavailable
- A fixture or the scenario's contract was ambiguous enough that no verdict is defensible
- Variance exceeds tolerance, or too few samples exist, for a close decision
- The revision/rep budget was exhausted before a stable result emerged

Declare per-scenario runtime requirements (e.g. "requires network access") in the scenario definition so their absence yields inconclusive rather than a graded fail. An inconclusive run is never disguised as a target failure and never disguised as a pass — explain the harness failure in the record, don't invent evidence to route around it. This is a Runtime-class failure (see above); it is excluded from pass-rate math and low-N counts, per grading.json's status field (owned by the grading pipeline, not this file).

## Micro-testing wording variants

Full scenario runs are the gate but are slow per iteration. Verify wording cheaply first:

1. One fresh-context sample per call — system prompt is the realistic context the guidance will live in, user message a task that tempts the failure
2. **Always include a no-guidance control.** If the control doesn't exhibit the failure, there is nothing to fix — stop; don't author the guidance
3. 5+ reps per variant; single samples lie
4. Manually read every flagged match — template echoes and quoted counter-examples masquerade as hits
5. Variance is a metric: five different interpretations across five reps means the wording isn't binding

Micro-tests verify wording; they never replace pressure scenarios for discipline skills.

## Rationalization capture format

Tier 3 runs produce this artifact; create-skill consumes it during REFACTOR. For every violation, record verbatim:

```json
{
  "scenario_id": "...",
  "chosen_option": "C",
  "expected_option": "A",
  "rationalizations": [
    "I already manually tested it",
    "Tests after achieve the same goals"
  ],
  "pressures_that_worked": ["sunk cost", "time"]
}
```

Each captured rationalization becomes, in the skill under construction: (1) an explicit negation in the rule, (2) a rationalization-table entry, (3) a red-flags list entry, and (4) where it signals an imminent violation, a symptom added to the description.

Overcompliance failures use the same artifact with a `"direction": "overapplied"` marker instead of `"rationalizations"` — record the over-application justification (why the agent thought the rule applied when it shouldn't have) rather than a rationalization for violating. These feed a scope-clarification, not a new negation — the fix narrows where the rule fires, it doesn't harden the rule further.
