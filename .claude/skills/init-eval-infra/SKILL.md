---
name: init-eval-infra
description: Use when a skill repo needs eval infrastructure bootstrapped for one of its skills — fixing the .claude/skills and .agents/skills symlink mounts and generating a starter runbooks/<id>.yaml so eval-skill can run against it. Trigger phrases include "set up eval infra for X", "scaffold a runbook for my skill", "make this skill repo eval-ready", "bootstrap eval infra". Requires create-skill and eval-skill to already be installed, and requires the target skill itself to already exist under skills/ (created via create-skill's init-skill). Does not author the eval content itself — trigger_evals[]/evals[] ship as TODO placeholders; writing real evals is eval-skill's job, guided by its references/scenario-design.md.
---

# Init Eval Infra

Bootstrap the eval side of the skill-forge / consumer-mirror layout for one
existing skill: fix the symlink mounts, then generate a starter
`runbooks/<id>.yaml`.

This skill is pure choreography over two already-installed CLIs —
`create-skill` and `eval-skill` — and has no CLI of its own.

## Steps

1. Confirm the target skill already exists at `skills/<name>/SKILL.md`.
   Missing → stop and point the user at `create-skill`'s `init-skill`
   subcommand instead; this skill only sets up the eval side, never the
   skill itself.
2. Fix the mounts. Resolve `create-skill`'s CLI per its own SKILL.md § CLI
   resolution, then run:
   ```
   <create-skill CLI> validate-mounts --repo-root . --fix
   ```
3. Generate the runbook. Resolve `eval-skill`'s CLI per its own SKILL.md §
   CLI resolution, then run:
   ```
   <eval-skill CLI> init-runbook <name> --repo-root .
   ```
   Pass `--id <id>` to name the runbook differently than the skill,
   `--tiers <csv>` to widen or narrow beyond the default `0,1`, and
   `--force` to overwrite an existing `runbooks/<id>.yaml`.
4. Report what changed, and point at the next step: `eval-skill`'s SKILL.md
   (Tier 1/2/3 sections) plus its `references/scenario-design.md` for
   turning the generated `TODO` placeholders into real evals.

## Not this skill's job

- Scaffolding `skills/<name>/SKILL.md` itself — that's `create-skill`'s
  `init-skill`.
- Authoring real eval content (prompts, expected outputs, discriminating
  trigger queries) — that's `eval-skill`, guided by its
  `references/scenario-design.md`.
