# `agents/openai.yaml` — Codex platform adapter

Read this before generating or editing `agents/openai.yaml`, the UI-metadata sidecar that lets a skill surface properly in Codex. The sidecar is a generated adapter artifact: it ships inside the skill (under `agents/`) so one directory serves both platforms, but the SKILL.md body stays platform-neutral and never depends on it. Generate and refresh it with `create-skill generate-openai-yaml` rather than hand-editing.

## Example

```yaml
interface:
  display_name: "Human-facing name"
  short_description: "A 25–64 character scanning description"
  default_prompt: "Use $skill-name to perform a representative task."

dependencies:
  tools:
    - type: "mcp"
      value: "github"
      description: "GitHub MCP server"
      transport: "streamable_http"
      url: "https://example.invalid/mcp/"
```

## Constraints

- Quote all string values; keep keys unquoted.
- Make `interface.display_name` concise and human-facing.
- Keep `interface.short_description` between 25 and 64 characters.
- Write `interface.default_prompt` as a short example prompt that explicitly mentions the skill as `$skill-name`.
- Resolve `interface.icon_small` and `interface.icon_large` relative to the skill directory, normally under `./assets/`.
- Use a six-digit hexadecimal value for `interface.brand_color`.
- Add icons and brand color only when the user supplies them; add `dependencies.tools` only for real runtime dependencies (currently `type: "mcp"`).
- Keep this metadata consistent with the capability and boundaries stated in SKILL.md's description — the sidecar restyles the description for a UI surface, it never widens or contradicts it.

## Supported interface fields

`display_name`, `short_description`, `default_prompt`, `icon_small`, `icon_large`, `brand_color`

## Supported MCP dependency fields

`type`, `value`, `description`, `transport`, `url`
