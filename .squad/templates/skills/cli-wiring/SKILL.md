# Skill: CLI Command Wiring

**Bug class:** Commands implemented in `packages/squad-cli/src/cli/commands/` but never routed in `cli-entry.ts`.

## Checklist — Adding a New CLI Command

1. **Create command file** in `packages/squad-cli/src/cli/commands/<name>.ts`
   - Export a `run<Name>(cwd, options)` async function (or class with static methods for utility modules)

2. **Add routing block** in `packages/squad-cli/src/cli-entry.ts` inside `main()`:
   ```ts
   if (cmd === '<name>') {
     const { run<Name> } = await import('./cli/commands/<name>.js');
     // parse args, call function
     await run<Name>(process.cwd(), options);
     return;
   }
   ```

3. **Add help text** in the help section of `cli-entry.ts` (search for `Commands:`):
   ```ts
   console.log(`  ${BOLD}<name>${RESET}     <description>`);
   console.log(`             Usage: <name> [flags]`);
   ```

4. **Verify both exist** — the recurring bug is doing step 1 but missing steps 2-3.

## Wiring Patterns by Command Type

| Type | Example | How to wire |
|------|---------|-------------|
| Standard command | `export.ts`, `build.ts` | `run*()` function, parse flags from `args` |
| Placeholder command | `loop`, `hire` | Inline in cli-entry.ts, prints pending message |
| Utility/check module | `rc-tunnel.ts`, `copilot-bridge.ts` | Wire as diagnostic check (e.g., `isDevtunnelAvailable()`) |
| Subcommand of another | `init-remote.ts` | Already used inside parent + standalone alias |

## Common Import Pattern

```ts
import { BOLD, RESET, DIM, RED, GREEN, YELLOW } from './cli/core/output.js';
```

Use dynamic `await import()` for command modules to keep startup fast (lazy loading).

## History

- **#237 / PR #244:** 4 commands wired (rc, copilot-bridge, init-remote, rc-tunnel). aspire, link, loop, hire were already present.
