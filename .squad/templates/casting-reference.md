# Casting Reference

On-demand reference for Squad's casting system. Loaded during Init Mode or when adding team members.

## Universe Table

| Universe | Capacity | Shape Tags | Resonance Signals |
|---|---|---|---|
| The Usual Suspects | 6 | small, noir, ensemble | crime, heist, mystery, deception |
| Reservoir Dogs | 8 | small, noir, ensemble | crime, heist, tension, loyalty |
| Alien | 8 | small, sci-fi, survival | space, isolation, threat, engineering |
| Ocean's Eleven | 14 | medium, heist, ensemble | planning, coordination, roles, charm |
| Arrested Development | 15 | medium, comedy, ensemble | dysfunction, business, family, satire |
| Star Wars | 12 | medium, sci-fi, epic | conflict, mentorship, legacy, rebellion |
| The Matrix | 10 | medium, sci-fi, cyberpunk | systems, reality, hacking, philosophy |
| Firefly | 10 | medium, sci-fi, western | frontier, crew, independence, smuggling |
| The Goonies | 8 | small, adventure, ensemble | exploration, treasure, kids, teamwork |
| The Simpsons | 20 | large, comedy, ensemble | satire, community, family, absurdity |
| Breaking Bad | 12 | medium, drama, tension | chemistry, transformation, consequence, power |
| Lost | 18 | large, mystery, ensemble | survival, mystery, groups, leadership |
| Marvel Cinematic Universe | 25 | large, action, ensemble | heroism, teamwork, powers, scale |
| DC Universe | 18 | large, action, ensemble | justice, duality, powers, mythology |
| Futurama | 12 | medium, sci-fi, comedy | future, robots, space, absurdity |

**Total: 15 universes** — capacity range 6–25.

## Selection Algorithm

Universe selection is deterministic. Score each universe and pick the highest:

```
score = size_fit + shape_fit + resonance_fit + LRU
```

| Factor | Description |
|---|---|
| `size_fit` | How well the universe capacity matches the team size. Prefer universes where capacity ≥ agent_count with minimal waste. |
| `shape_fit` | Match universe shape tags against the assignment shape derived from the project description. |
| `resonance_fit` | Match universe resonance signals against session and repo context signals. |
| `LRU` | Least-recently-used bonus — prefer universes not used in recent assignments (from `history.json`). |

Same inputs → same choice (unless LRU changes between assignments).

## Casting State File Schemas

### policy.json

Source template: `.squad/templates/casting-policy.json`
Runtime location: `.squad/casting/policy.json`

```json
{
  "casting_policy_version": "1.1",
  "allowlist_universes": ["Universe Name", "..."],
  "universe_capacity": {
    "Universe Name": 10
  }
}
```

### registry.json

Source template: `.squad/templates/casting-registry.json`
Runtime location: `.squad/casting/registry.json`

```json
{
  "agents": {
    "agent-role-id": {
      "persistent_name": "CharacterName",
      "universe": "Universe Name",
      "created_at": "ISO-8601",
      "legacy_named": false,
      "status": "active"
    }
  }
}
```

### history.json

Source template: `.squad/templates/casting-history.json`
Runtime location: `.squad/casting/history.json`

```json
{
  "universe_usage_history": [
    {
      "universe": "Universe Name",
      "assignment_id": "unique-id",
      "used_at": "ISO-8601"
    }
  ],
  "assignment_cast_snapshots": {
    "assignment-id": {
      "universe": "Universe Name",
      "agents": {
        "role-id": "CharacterName"
      },
      "created_at": "ISO-8601"
    }
  }
}
```
