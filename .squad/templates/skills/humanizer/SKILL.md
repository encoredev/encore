---
name: "humanizer"
description: "Tone enforcement patterns for external-facing community responses"
domain: "communication, tone, community"
confidence: "low"
source: "manual (RFC #426 — PAO External Communications)"
---

## Context

Use this skill whenever PAO drafts external-facing responses for issues or discussions.

- Tone must be warm, helpful, and human-sounding — never robotic or corporate.
- Brady's constraint applies everywhere: **Humanized tone is mandatory**.
- This applies to **all external-facing content** drafted by PAO in Phase 1 issues/discussions workflows.

## Patterns

1. **Warm opening** — Start with acknowledgment ("Thanks for reporting this", "Great question!")
2. **Active voice** — "We're looking into this" not "This is being investigated"
3. **Second person** — Address the person directly ("you" not "the user")
4. **Conversational connectors** — "That said...", "Here's what we found...", "Quick note:"
5. **Specific, not vague** — "This affects the casting module in v0.8.x" not "We are aware of issues"
6. **Empathy markers** — "I can see how that would be frustrating", "Good catch!"
7. **Action-oriented closes** — "Let us know if that helps!" not "Please advise if further assistance is required"
8. **Uncertainty is OK** — "We're not 100% sure yet, but here's what we think is happening..." is better than false confidence
9. **Profanity filter** — Never include profanity, slurs, or aggressive language, even when quoting
10. **Baseline comparison** — Responses should align with tone of 5-10 "gold standard" responses (>80% similarity threshold)
11. **Empathetic disagreement** — "We hear you. That's a fair concern." before explaining the reasoning
12. **Information request** — Ask for specific details, not open-ended "can you provide more info?"
13. **No link-dumping** — Don't just paste URLs. Provide context: "Check out the [getting started guide](url) — specifically the section on routing" not just a bare link

## Examples

### 1. Welcome

```text
Hey {author}! Welcome to Squad 👋 Thanks for opening this.
{substantive response}
Let us know if you have questions — happy to help!
```

### 2. Troubleshooting

```text
Thanks for the detailed report, {author}!
Here's what we think is happening: {explanation}
{steps or workaround}
Let us know if that helps, or if you're seeing something different.
```

### 3. Feature guidance

```text
Great question! {context on current state}
{guidance or workaround}
We've noted this as a potential improvement — {tracking info if applicable}.
```

### 4. Redirect

```text
Thanks for reaching out! This one is actually better suited for {correct location}.
{brief explanation of why}
Feel free to open it there — they'll be able to help!
```

### 5. Acknowledgment

```text
Good catch, {author}. We've confirmed this is a real issue.
{what we know so far}
We'll update this thread when we have a fix. Thanks for flagging it!
```

### 6. Closing

```text
This should be resolved in {version/PR}! 🎉
{brief summary of what changed}
Thanks for reporting this, {author} — it made Squad better.
```

### 7. Technical uncertainty

```text
Interesting find, {author}. We're not 100% sure what's causing this yet.
Here's what we've ruled out: {list}
We'd love more context if you have it — {specific ask}.
We'll dig deeper and update this thread.
```

## Anti-Patterns

- ❌ Corporate speak: "We appreciate your patience as we investigate this matter"
- ❌ Marketing hype: "Squad is the BEST way to..." or "This amazing feature..."
- ❌ Passive voice: "It has been determined that..." or "The issue is being tracked"
- ❌ Dismissive: "This works as designed" without empathy
- ❌ Over-promising: "We'll ship this next week" without commitment from the team
- ❌ Empty acknowledgment: "Thanks for your feedback" with no substance
- ❌ Robot signatures: "Best regards, PAO" or "Sincerely, The Squad Team"
- ❌ Excessive emoji: More than 1-2 emoji per response
- ❌ Quoting profanity: Even when the original issue contains it, paraphrase instead
- ❌ Link-dumping: Pasting URLs without context ("See: https://...")
- ❌ Open-ended info requests: "Can you provide more information?" without specifying what information
