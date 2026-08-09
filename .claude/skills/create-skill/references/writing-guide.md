# Skill Writing Guide

How to write skill content that triggers correctly, reads efficiently, and survives contact with real agents. Checkable versions of these rules: eval-skill's `references/quality-standards.md` (rule IDs D1–D8, W1–W7, R1–R5).

## Contents

- [Description: the triggering contract](#description-the-triggering-contract)
- [Choosing the body structure](#choosing-the-body-structure)
- [Completion criteria](#completion-criteria)
- [Body style](#body-style)
- [Leading words](#leading-words)
- [Degrees of freedom](#degrees-of-freedom)
- [Match the form to the failure](#match-the-form-to-the-failure)
- [Bulletproofing discipline skills](#bulletproofing-discipline-skills)
- [Examples](#examples)
- [Flowcharts](#flowcharts)
- [Cross-referencing](#cross-referencing)
- [Bundled script quality](#bundled-script-quality)
- [Anti-patterns](#anti-patterns)

## Description: the triggering contract

The description is read by the harness ABOUT the skill, for every conversation. It decides whether the body ever loads.

**Third person.** "This skill should be used when…" or "Use when…" — never "I can help you…" or "You should use this…".

**Capability at intent level + concrete triggers.** Say what the skill covers and when to reach for it: user phrases (verbatim trigger phrases work), symptoms, error messages, file types, situations. All "when to use" information lives here — a "When to Use" section in the body loads too late to influence triggering.

**NEVER summarize the workflow.** This rule has empirical teeth: a description saying "…with code review between tasks" caused agents to do ONE review even though the body's flowchart clearly showed TWO — the agent followed the description's summary instead of reading the body. Descriptions that summarize process become shortcuts that make the body dead weight. Describe the problem the skill solves, never the steps it takes.

```yaml
# BAD: workflow summary — agents follow this instead of reading the skill
description: Use for TDD - write test first, watch it fail, write minimal code, refactor

# BAD: vague, no triggers
description: Provides guidance for working with hooks.

# BAD: artificially pushy
description: ... Make sure to use this whenever the user mentions data, even if they don't ask.

# GOOD: capability + triggering conditions, no process
description: This skill should be used when tests have race conditions, timing dependencies, or pass/fail inconsistently. Covers async test stabilization for any framework.
```

**No pushiness.** "Use this even when the user doesn't ask" inflates trigger rates by stealing attention from sibling skills. If triggering is weak, fix it with measured trigger evals (eval-skill Tier 1), not with prose pressure.

**Boundary sentence — only when needed.** When a common near-miss would otherwise select the skill (a sibling covers the adjacent job), end the description with one "Not for X — use Y" sentence. Don't enumerate exclusions speculatively; each one costs trigger surface.

**Keyword coverage.** Include the words an agent would match on: exact error messages ("ENOTEMPTY", "Hook timed out"), symptoms ("flaky", "hanging"), synonyms (timeout/hang/freeze), tool and library names. Keep triggers technology-agnostic unless the skill is technology-specific — then name the technology explicitly.

**For discipline skills:** include symptoms that the agent is ABOUT to violate the rule ("use when tempted to skip tests", "when manual testing seems faster").

## Choosing the body structure

Pick the structure that makes the agent's next decision obvious. Most substantial skills use one dominant structure plus at most a secondary section where it genuinely reduces ambiguity:

| Structure | Use for | Shape |
|---|---|---|
| Workflow | Ordered processes with gates or feedback loops | Overview → step → validation → step → recovery; state which steps may be skipped and on what evidence |
| Task | A collection of distinct operations | Quick start → one section per task; shared invariants stated once near the top, never repeated per task |
| Reference | Standards, schemas, domain knowledge | Selection and application guidance in SKILL.md; the material itself in `references/` |
| Capability | Cooperating features with no fixed sequence | Each capability: inputs, output, and how it composes with the others |

## Completion criteria

End every workflow step on a condition the agent can check, not a feeling of progress. Two properties make a criterion strong:

- **Checkable** — the agent can tell done from not-done: "all placeholders replaced and the linter passes", not "content improved".
- **Exhaustive where thoroughness matters** — the bound forces the full sweep: "every modified model accounted for", not "produce a change list". Exhaustive bounds also bind reference-structure skills — "every rule applied" does for a review what "every step done" does for a sequence.

A vague criterion invites premature completion: the agent's attention slips from the work to being done, and the visible later steps pull it forward. Fix in order — sharpen the criterion first (cheap, local); restructure to hide the later steps only when the criterion is irreducibly fuzzy AND transcripts actually show the rush. Hiding requires a real context boundary (a reference file not yet loaded, a subagent hand-off): later sections of the same SKILL.md are already in context and hide nothing.

## Body style

- **Imperative form.** "Run the validator", not "You should run…" — the body is instructions to an agent, not conversation.
- **Explain the why.** Models have good theory of mind; a rule with its reason travels further than a bare command. Writing ALWAYS or NEVER in all caps is a yellow flag: reframe so the model understands why it matters. (Exception: discipline skills that failed pressure testing may need hard prohibitions — see Match the form to the failure.)
- **Lean by default.** Don't explain what's obvious from a command. Don't repeat cross-referenced content. Move flag documentation to `--help`. When iterating, read the run transcripts: content that makes the model do unproductive work gets cut.
- **The no-op test, per sentence.** Does this sentence change behavior versus what the model already does by default? A sentence can be true and relevant and still be a no-op; delete the whole sentence rather than trimming words from it. The verdict is model-relative — disagreement about a no-op is disagreement about the default, settled by an eval run, not debate.
- **Fresh-eyes pass.** Draft, then reread as if you've never seen it. Every section must answer "what does the agent DO with this?" Sweep for semantic restatements across workflow, quality-gate, and resource sections — each directive lives at one decision point; a file catalog may say what a file contains but never repeats when or how to use it.

## Leading words

A leading word is a compact concept the model already carries from pretraining — _tracer bullets_, _fog of war_, _relentless_ — placed so the agent thinks with it while running the skill. One strong token recruits priors a paragraph would have to spell out, and it anchors twice: in the body it anchors execution (the agent reaches for the same behavior every time the word appears); in the description it anchors invocation (when the word also lives in the user's own prompts and docs, the skill fires more reliably).

Treat it as the compression move during refactoring: a quality restated across a phase ("fast, deterministic, low-overhead") collapses into one pretrained word (a _tight_ loop); a fuzzy gate ("a loop you believe in") becomes a binary observable (the loop goes _red_). Fewer tokens and a sharper hook. Prefer words the model already knows — a coined term recruits no priors, so it costs the definition tokens a pretrained word gets free.

Grade a leading word with the no-op test: _be thorough_ when the model is already thorough-ish changes nothing — the fix is a stronger word (_relentless_), not more sentences.

## Degrees of freedom

Match specificity to the task's fragility — think of the agent exploring a path: an open field allows many routes; a narrow bridge with cliffs needs guardrails.

| Freedom | Form | Use when |
|---------|------|----------|
| High | Text heuristics | Multiple approaches valid; decisions depend on context |
| Medium | Pseudocode or parameterized scripts | A preferred pattern exists; some variation acceptable |
| Low | Locked scripts, few parameters | Operations fragile or error-prone; consistency critical |

## Match the form to the failure

Classify the baseline failure BEFORE writing guidance — the form that fixes one failure type measurably backfires on another:

| Baseline failure | Right form | Wrong form |
|---|---|---|
| Skips/violates a rule under pressure (knows better, does it anyway) | Prohibition + rationalization table + red flags | Soft guidance ("prefer…", "consider…") |
| Complies, but output has the wrong shape | Positive recipe/contract: state what the output IS — its parts, in order | Prohibition list ("don't restate…") |
| Omits a required element from something they already produce | Structural: REQUIRED slot in the template they fill | Prose reminders near the template |
| Behavior should depend on a condition | Conditional keyed to an observable predicate | Unconditional rule + exemption clauses |

Why prohibitions backfire on shaping problems: under a competing incentive, agents negotiate with "don't X". In head-to-head wording tests, the prohibition arm produced MORE of the unwanted content than the recipe arm — and trended worse than no guidance at all. A recipe leaves nothing to negotiate.

Two rules regardless of form:

- **No nuance clauses.** "Don't X unless it matters" reopens the negotiation; a single nuance clause degraded a winning recipe to noise in wording tests. Express real exceptions as their own conditionals on observable predicates.
- **Exemption clauses don't scope.** "This limit doesn't apply to code blocks" still suppresses code blocks. Restructure so the rule can't reach the exempt part.

## Bulletproofing discipline skills

For rules agents will want to break under pressure. Build these from Tier 3 eval evidence (verbatim rationalizations), not from imagination.

1. **Close every loophole explicitly.** "Delete it" becomes: "Delete it. Don't keep it as 'reference'. Don't 'adapt' it while writing tests. Don't look at it. Delete means delete."
2. **Foundational principle early:** "Violating the letter of the rules is violating the spirit of the rules." Cuts off the entire spirit-vs-letter class.
3. **Rationalization table.** Every excuse captured during pressure testing gets an Excuse | Reality row.
4. **Red flags list.** The agent's own likely internal monologue, quoted: "I already manually tested it", "This is different because…" — each resolving to one deterministic action.
5. **Gate functions** for checkable decision points:

   ```
   BEFORE writing the mock:
     1. Do I understand what this dependency actually returns? If no → STOP, go run it.
     2. Does the test need the full structure? If yes → mirror it completely.
   ```

6. **Description symptoms** — add "about to violate" signals to the frontmatter description.

Persuasion research (Meincke et al. 2025, N≈28k: persuasion principles raised compliance 33%→72%) maps to skill writing: **authority** (unambiguous rules), **commitment** (make the agent state the rule before acting), and **social proof** ("every skipped test in this codebase's history caused a regression") reinforce discipline skills; avoid liking/reciprocity framings ("please", "it would help me") — they invite negotiation.

## Examples

One excellent example beats many mediocre ones. Complete, runnable, from a realistic scenario, commented for WHY. Choose the single most relevant language — agents port well. Never: multi-language example sets, fill-in-the-blank templates, contrived toy cases.

For input/output formats, show a worked pair:

```markdown
**Example:**
Input: Added user authentication with JWT tokens
Output: feat(auth): implement JWT-based authentication
```

## Flowcharts

Mermaid, and only for genuinely non-obvious decision points: "A vs B" choices, loops where the agent might stop too early. Never for linear sequences (numbered lists), reference material (tables), or code (code blocks). Nodes carry semantic labels, not step1/step2.

## Cross-referencing

- Other skills: by name with a requirement marker — `REQUIRED BACKGROUND: understand <skill-name> before using this skill`.
- Bundled files: relative path plus WHEN to read it — "See `references/forms.md` when the task involves fillable forms." For scripts, also say whether the agent should execute the script, read it as an example, or both.
- The pointer's wording, not its target, decides whether the file is ever reached: a must-read file behind a weakly worded pointer is a variance bug. Sharpen the pointer first; inline the material only if sharpening fails.
- Never `@`-link files: Claude Code's `@` syntax force-loads the target into context immediately, defeating progressive disclosure. (Other platforms lack the syntax; the plain-path rules above work everywhere.)

## Bundled script quality

- **Test by running.** Every shipped script has been executed against a realistic input (a representative sample when many scripts are similar). "It looks right" is not tested.
- **Solve, don't punt.** No placeholder logic the agent must fill in at run time; handle the error cases the skill's own examples will hit.
- **No voodoo constants.** Every threshold or magic number gets a comment saying where it came from.
- **Deterministic beats clever.** Scripts exist to remove variance; keep flags few and defaults safe.

## Anti-patterns

| Anti-pattern | Why it fails |
|---|---|
| Narrative storytelling ("In session 2025-10-03 we found…") | Too specific to reuse; agents need the technique, not the war story |
| Multi-language example dilution | Mediocre quality × N maintenance burdens |
| Workflow summary in description | Agents execute the summary and skip the body (see Description) |
| Second-person body ("You should…") | Conversational register, weaker instruction-following |
| Unreferenced bundled files | Invisible to the agent — they will never be read |
| Duplicated content in SKILL.md and references | Two copies drift; the agent reads the stale one |
| Code inside flowcharts | Can't copy-paste; hard to read |
| Generic labels (helper1, step3) | Labels must carry meaning |
