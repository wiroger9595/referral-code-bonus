# Cross-Runtime Compatibility

Use this reference when a skill must run in more than one Agent Skills client, especially Codex and Claude Code.

## Contents

- [Choose a target profile](#choose-a-target-profile)
- [Keep the core portable](#keep-the-core-portable)
- [Separate platform adapters](#separate-platform-adapters)
- [Share one source inside a repository](#share-one-source-inside-a-repository)
- [Validate both runtimes](#validate-both-runtimes)
- [Skills that ship a CLI binary](#skills-that-ship-a-cli-binary)

## Choose a Target Profile

Choose the profile before scaffolding or revising the skill:

| Profile | Use when | Core `SKILL.md` | Platform files |
| --- | --- | --- | --- |
| `portable` | The client is not yet known | Agent Skills standard only | None by default |
| `codex` | Only Codex must load the skill | Standard core | `agents/openai.yaml` when UI metadata is useful |
| `claude` | Only Claude Code must load the skill | Standard core plus deliberate Claude extensions | None required |
| `both` | Codex and Claude Code must share one source | Portable intersection | Codex sidecar is allowed; avoid Claude-only core syntax |

Default to `both` only when cross-runtime use is part of the contract. Do not silently remove a platform-specific capability from an existing skill merely to make it portable.

## Keep the Core Portable

For the `portable` and `both` profiles:

- Require `name` and `description` in YAML frontmatter.
- Prefer only `name` and `description`, the verified cross-client intersection.
- If an optional frontmatter field is necessary (`license`, `metadata`, or `allowed-tools` — the set eval-skill's Tier 0 lint accepts), verify it in every target client rather than assuming identical handling. Express client-compatibility notes inside `metadata` rather than a top-level `compatibility` key, which the lint rejects.
- Treat `allowed-tools` as experimental and verify both clients before relying on it for authorization.
- Keep workflow instructions imperative and use relative links from the skill root.
- Refer to the runtime generically as the agent, host, client, executor, or grader unless behavior genuinely differs.
- Pass paths and arguments through normal task instructions or scripts with explicit parameters.

Do not place these Claude Code extensions in a shared core unless Codex behavior has been separately verified:

- invocation fields such as `disable-model-invocation`, `user-invocable`, `context`, or `agent`;
- Claude-only argument and environment substitutions such as `$ARGUMENTS` or `${CLAUDE_SKILL_DIR}`;
- dynamic shell injection with ``!`command` ``;
- skill-scoped hooks, model selection, path gating, or Claude tool restrictions.

Do not put Codex UI policy or dependencies in `SKILL.md`; keep them in `agents/openai.yaml` (field rules: [openai-yaml.md](openai-yaml.md)).

When a required feature has no portable equivalent, keep the portable workflow in the shared skill and create the smallest platform-specific wrapper or adapter. Test the wrapper as a separate configuration. Do not make another complete copy of the skill body.

## Separate Platform Adapters

The shared skill owns outcomes, decisions, resources, and validation. A platform adapter owns discovery and host-specific execution.

### Codex

- Discover repository skills from `.agents/skills/<name>/`.
- Invoke explicitly as `$name` when needed.
- Put UI metadata, implicit-invocation policy, and declared tool dependencies in `agents/openai.yaml`.
- Use isolated `codex exec` processes (preferred) or host collaboration agents for evaluation.

### Claude Code

- Discover repository skills from `.claude/skills/<name>/`.
- Invoke explicitly as `/name` when needed.
- Use Claude-only frontmatter or substitutions only for a Claude-specific profile or wrapper.
- Use isolated `claude -p` processes (preferred) or native subagents for evaluation.

Never write both invocation forms into a user's task prompt merely to make one prompt work everywhere. Let the host adapter bind the skill while preserving the task text.

## Share One Source Inside a Repository

Keep each skill once in a canonical `skills/` directory and expose it through both project discovery roots with relative symlinks:

```text
repo/
├── skills/
│   ├── create-skill/
│   └── eval-skill/
├── .agents/
│   └── skills/
│       ├── create-skill -> ../../skills/create-skill
│       └── eval-skill -> ../../skills/eval-skill
└── .claude/
    └── skills/
        ├── create-skill -> ../../skills/create-skill
        └── eval-skill -> ../../skills/eval-skill
```

Commit relative directory symlinks so clones remain relocatable. Resolve each link and verify that its final directory contains the expected `SKILL.md`. Never edit a skill through a discovery-root path — resolve to the canonical `skills/<name>/` and edit there. Do not install or copy the skills into a user's home directory when repository scope is requested.

Symlink support is a runtime prerequisite. If a target client version cannot follow project skill symlinks, stop and report the compatibility limitation rather than creating an untracked duplicate.

## Validate Both Runtimes

Run static validation on the skill itself (eval-skill's Tier 0 lint), then validate the repository mounts with the `create-skill` command resolved per SKILL.md § CLI resolution:

```bash
create-skill validate-mounts --repo-root <repo>
```

`validate-mounts` checks that every skill under `skills/` has a relative, repo-internal symlink in each discovery root, that both roots resolve to the same canonical directory, and that the resolved `SKILL.md` hash matches the canonical source.

Then verify:

1. both repository links resolve to the same physical skill directory;
2. Codex lists and explicitly loads the skill from `.agents/skills`;
3. Claude Code lists and explicitly loads the skill from `.claude/skills`;
4. positive and near-miss discovery prompts are tested separately in each runtime (eval-skill Tier 1, `--cli=claude` and `--cli=codex`);
5. platform-specific behavior is reported in separate result strata rather than averaged together.

Passing one client proves only the portable file shape, not equivalent discovery or behavior in the other client.

## Skills that ship a CLI binary

A skill that distributes a compiled CLI (the shape `create-skill package-skill --platform` produces) ships no launcher script. Instead, its SKILL.md opens with a short "CLI resolution" section the reading agent follows once per session:

1. A platform binary exists in `bin/`? (`bin/<name>-darwin-arm64` on macOS, `bin/<name>-linux-x64` on Linux, `bin/<name>-windows-x64.exe` on Windows) → use that path.
2. Otherwise → run the TypeScript source under Bun (state the source path and, when the skill can run sandboxed, the env var that locates the repo root).
3. Neither exists → report the CLI unavailable and stop.

Spell every command in the rest of the document with a bare alias (`<name> <subcommand> …`) defined by that section, so the body stays platform-neutral and the resolution rule lives in exactly one place. Platform selection is the agent's explicit job at read time — the same philosophy as the explicit `--cli=` flag, with no auto-detecting shim in between.
