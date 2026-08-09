# Skill Anatomy

Structure, naming, and context-budget rules for a skill directory. The checkable versions of these rules live in eval-skill's `references/quality-standards.md` (rule IDs S1–S8, B1–B6); this file explains how to apply them while building.

## Contents

- [What a skill is](#what-a-skill-is)
- [Directory layout](#directory-layout)
- [Progressive disclosure](#progressive-disclosure)
- [The three resource types](#the-three-resource-types)
- [Frontmatter policy](#frontmatter-policy)
- [Naming](#naming)
- [Reference-file hygiene](#reference-file-hygiene)
- [What NOT to include](#what-not-to-include)
- [Structural templates](#structural-templates)

## What a skill is

A skill is an onboarding guide for a domain or task — it turns a general-purpose agent into a specialist by supplying procedural knowledge, domain context, and reusable tools the model doesn't already have.

Default assumption: the model is already smart. Only add what it doesn't know. Challenge every piece of content: "Does this paragraph justify its token cost?" The context window is a public good shared with the system prompt, the conversation, and every other skill's metadata.

Skills are reference guides for proven techniques — not narratives about how a problem was solved once.

## Directory layout

```
skill-name/
├── SKILL.md                 (required)
│   ├── YAML frontmatter     (name + description)
│   └── Markdown body        (imperative instructions)
└── Bundled resources        (optional)
    ├── scripts/             executable code
    ├── references/          docs loaded into context on demand
    └── assets/              files used in output, never loaded
```

The skill's versioned eval suite does NOT live inside the skill directory — it lives in the repo's runbook (`runbooks/<skill-name>.yaml`, contract: eval-skill's schemas reference § runbook). Extra files inside a skill alter discovery and packaging; the repo owns its eval history.

## Progressive disclosure

Three loading levels; each level costs context only when reached:

1. **Metadata** (name + description) — always in context, for every conversation, whether or not the skill is used (~100 words)
2. **SKILL.md body** — loaded only when the skill triggers (≤ 500 lines / < 5k words hard ceiling; 1,500–2,000 words working target)
3. **Bundled resources** — loaded as needed; effectively unlimited because scripts can be *executed without ever entering the context window*

Progressive disclosure is the size-control mechanism. When the body approaches the ceiling, push detail into `references/` with clear pointers — don't amputate content, and don't exceed the ceiling.

The counter-rule: don't split a short linear procedure into references merely to cut line count — every hop the agent must take is context cost too. The branch test decides placement: keep inline what every path through the skill needs; push behind a pointer what only some branches reach — plus anything large or executable regardless of branch.

## The three resource types

**`scripts/`** — executable code for operations that are fragile, deterministic, or repeatedly rewritten. Benefits: token-efficient (runs without loading), deterministic. Scripts must be tested by actually running them before shipping. A script earns its place empirically: if eval-run agents independently rewrote the same helper (`create_docx.py` three times in three transcripts), bundle it.

**`references/`** — documentation loaded on demand: schemas, API docs, detailed workflow guides, domain knowledge. Keeps SKILL.md lean while keeping information discoverable. Never duplicate content between SKILL.md and a reference file — information lives in exactly one place. Prefer stable principles over volatile facts: when a fact can drift (versions, quotas, endpoints), point at the authoritative source to check instead of embedding a dated snapshot as timeless truth.

**`assets/`** — files used in the output, never loaded into context: templates, boilerplate projects, fonts, logos.

Decide what to bundle from the concrete examples gathered during intent capture: for each example, mentally execute it from scratch and ask what would be re-written or re-discovered every time. That artifact is what to bundle. (PDF rotation → `scripts/rotate_pdf.py`; webapp boilerplate → `assets/hello-world/`; BigQuery schemas → `references/schema.md`.)

## Frontmatter policy

```yaml
---
name: skill-name
description: This skill should be used when ...
---
```

- `name` and `description` are the required, agent-agnostic core. The optional allow-listed extras are `license`, `allowed-tools`, `metadata` — platform adapters may use them; nothing else is legal.
- The description is the skill's ONLY triggering mechanism. Writing rules: `writing-guide.md` § Description.
- Platform-specific sidecars (e.g. a Codex `agents/openai.yaml`) are generated adapter artifacts the SKILL.md body never depends on — shipping policy and field rules: `references/openai-yaml.md`.

## Naming

- Lowercase letters, digits, hyphens only; ≤ 64 chars; directory name equals `name` exactly.
- Short, verb-led phrases describing the action: `create-skill`, `condition-based-waiting`, `root-cause-tracing`. Gerunds work well for processes.
- Name by what the skill DOES or its core insight, not by category: `flatten-with-flags` beats `data-structure-refactoring`.
- Namespace by tool when it improves clarity or triggering: `gh-address-comments`.

## Reference-file hygiene

- Keep references **one level deep** from SKILL.md, and link every one of them from SKILL.md with guidance on WHEN to read it — an unreferenced file is invisible to the agent.
- Files > 100 lines: include a table of contents at the top.
- Files > 10k words: include grep search patterns in SKILL.md so the agent can search instead of loading.
- Multi-domain or multi-variant skills: organize references by domain/variant (`references/aws.md`, `references/gcp.md`, …) with only the workflow and selection guidance in SKILL.md — the agent then loads only the relevant variant.

## What NOT to include

No auxiliary meta-documentation: README.md, INSTALLATION_GUIDE.md, QUICK_REFERENCE.md, CHANGELOG.md, or anything about the process of creating the skill. A skill contains only what an agent needs to do the job. Everything else is clutter that competes for attention.

## Structural templates

**Minimal** — simple knowledge, no resources:

```
skill-name/
└── SKILL.md
```

**Standard** — most skills:

```
skill-name/
├── SKILL.md
└── references/
    └── detailed-guide.md
```

(plus `runbooks/skill-name.yaml` at the repo root carrying the eval suite)

**Complete** — complex domains with tooling:

```
skill-name/
├── SKILL.md
├── references/
│   ├── patterns.md
│   └── advanced.md
├── scripts/
│   └── validate.py
└── assets/
    └── template/
```

Create only the directories the skill actually needs.
