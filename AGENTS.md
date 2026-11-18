<!-- fleet-agents-pointer:begin -->
# ⚠️ FLEET RULES — LOAD ~/.agents/AGENTS.md FIRST

Before applying any rule in this file, load and follow `~/.agents/AGENTS.md`
— the fleet-level rules file every machine carries (synced from
`arrrrny/skills`, the single source of truth, by `sync-skills` inside
`kimi-code-update`, 3x/day). It owns the fleet-wide HARD RULES: the
skills-sync rule (every merge-PR task must check the repo's exposed skills —
create or update them on exported-behavior changes — and auto-PR them to
`arrrrny/skills`), the English-only response rule, the GitHub account
spelling (`arrrrny` = a + exactly 4 r's + ny), and the machine bootstrap
contract (`arrrrny/scripts` + `arrrrny/skills`). Future fleet-wide rule
changes are made ONCE in that file — never by editing every repo. Where this
file and the fleet file disagree, the fleet file wins unless this file is
MORE restrictive for this repo.
<!-- fleet-agents-pointer:end -->
# Project testing rules

- Before implementation, produce a complete architecture design covering the domain model, ownership and lifecycle, data flow, persistence, APIs, frontend state, failure recovery, migration, and compatibility with existing Solo behavior.
- Review every change from the whole-project perspective. Trace the existing frontend, server, daemon, local agent runtime, session, and database paths before changing one feature path.
- All new product-flow validation must use real components and dependencies; do not use mocks.
- End-to-end tests must use the real frontend, API server, and PostgreSQL database.
- Do not mock HTTP routes, backend services, or database behavior.
- Validation must cover frontend behavior and the complete backend path, including the real local agent runtime when the feature invokes agents; UI-only tests are not sufficient.
- Report E2E success only after verifying the user-visible result and persisted database state.

# Project service lifecycle rules

- An agent may start or restart the Solo frontend, API server, and daemon when requested, but only by running `make rebuild` from the repository root. Tests must reuse that make-managed stack.
- Never create `launchctl` jobs or manually start these services with direct binaries, `go run`, `npm run dev`, `nohup`, or custom background commands.
- When the user asks to stop the project, use `make stop` and verify that ports 3000, 8080, and 8081 are no longer listening. Do not restart services unless the user explicitly asks.