# Personal Squad — Skill Document

## What is a Personal Squad?

A personal squad is a user-level collection of AI agents that travel with you across projects. Unlike project agents (defined in a project's `.squad/` directory), personal agents live in your global config directory and are automatically discovered when you start a squad session.

## Directory Structure

```
~/.config/squad/personal-squad/    # Linux/macOS
%APPDATA%/squad/personal-squad/    # Windows
├── agents/
│   ├── {agent-name}/
│   │   ├── charter.md
│   │   └── history.md
│   └── ...
└── config.json                    # Optional: personal squad config
```

## How It Works

1. **Ambient Discovery:** When Squad starts a session, it checks for a personal squad directory
2. **Merge:** Personal agents are merged into the session cast alongside project agents
3. **Ghost Protocol:** Personal agents can read project state but not write to it
4. **Kill Switch:** Set `SQUAD_NO_PERSONAL=1` to disable ambient discovery

## Commands

- `squad personal init` — Bootstrap a personal squad directory
- `squad personal list` — List your personal agents
- `squad personal add {name} --role {role}` — Add a personal agent
- `squad personal remove {name}` — Remove a personal agent
- `squad cast` — Show the current session cast (project + personal)

## Ghost Protocol

See `templates/ghost-protocol.md` for the full rules. Key points:
- Personal agents advise; project agents execute
- No writes to project `.squad/` state
- Transparent origin tagging in logs
- Project agents take precedence on conflicts

## Configuration

Optional `config.json` in the personal squad directory:
```json
{
  "defaultModel": "auto",
  "ghostProtocol": true,
  "agents": {}
}
```

## Environment Variables

- `SQUAD_NO_PERSONAL` — Set to any value to disable personal squad discovery
- `SQUAD_PERSONAL_DIR` — Override the default personal squad directory path
