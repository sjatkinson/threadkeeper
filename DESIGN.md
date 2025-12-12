# ThreadKeeper – Design Notes

This document captures the **design intent, constraints, and guiding decisions** behind ThreadKeeper.

It is not an implementation guide.
It exists to explain *why* things are shaped the way they are, and what tradeoffs are intentional.

If you find yourself asking “why doesn’t ThreadKeeper do X?”, the answer is probably here.

---

## Design Goals

ThreadKeeper is designed to be:

- **Local-first**
- **Durable over time**
- **Fast and low-friction**
- **Human-scale**
- **Text-centric**

Every design decision should be evaluated against these goals.

---

## Core Concepts

### Tasks as Threads

The central abstraction in ThreadKeeper is a **thread**.

A thread:
- Represents ongoing attention, not just a unit of work
- Can accumulate context over time
- May pause, resume, or end without “completion”

Tasks are treated as *living objects*, not static records.

---

### Notes Are Context, Not Knowledge

ThreadKeeper supports notes, but with a strict scope:

- Notes exist **to support tasks**
- Notes are attached to threads
- Notes capture thinking, state, and history

ThreadKeeper deliberately avoids becoming a general-purpose knowledge base.

If a note no longer relates to an active or remembered task, it probably belongs elsewhere.

---

### Identity and Stability

Tasks require **stable identities**.

Design intent:
- Task IDs should never change
- IDs must survive edits, reordering, and deletion
- IDs should work across machines and backups

Human-readable names are for display.  
Stable identifiers are for durability.

---

## Scope and Non-Goals

### Explicit Non-Goals

ThreadKeeper will not attempt to provide:

- Project management workflows
- Multi-user concurrency or permissions
- Rich visualizations or dashboards
- AI-driven task decomposition
- Deep calendar or scheduling logic

These features add complexity that undermines the core goals.

---

### Team Use (Limited)

ThreadKeeper may support:
- One person
- A handful of trusted collaborators

It is not designed for:
- Large teams
- Centralized servers
- Real-time collaboration

Any “distributed” usage assumes trust and low contention.

---

## Interface Philosophy

### CLI First

The command line is the primary interface.

Rationale:
- Scriptable
- Fast
- Stable over time
- Editor-friendly

Other interfaces (TUI, GUI) are optional and secondary.

---

### Editor as the Primary UI

ThreadKeeper assumes:
- Users are comfortable in an editor
- Long-form edits belong in `$EDITOR`
- Structured data should remain human-readable

If something requires a custom UI to be usable, it is probably too complex.

---

## Storage and Durability

### Predictable Storage

Design principles:
- On-disk formats should be inspectable
- Data should be resilient to partial corruption
- Backups should be trivial

ThreadKeeper favors boring, transparent storage over opaque databases.

---

### Versioning and History

ThreadKeeper acknowledges that:
- Mistakes happen
- Context changes
- Rollback is sometimes necessary

Design direction:
- Support external version control
- Avoid hidden state
- Prefer append-friendly data models

---

## Tagging and Organization

Tags are:
- Lightweight
- Optional
- Non-hierarchical

ThreadKeeper does not enforce:
- Tag schemas
- Controlled vocabularies
- Global taxonomies

The system should tolerate inconsistency rather than impose structure.

---

## Attachments and References

Attachments may include:
- Files
- Links
- External references

Design tension:
- Embedded storage vs. linking
- Portability vs. simplicity

The system should avoid forcing a single model too early.

---

## Editing Model

Editing should support:
- Incremental updates
- Bulk changes
- Deferred structure

ThreadKeeper avoids:
- Mandatory fields beyond the basics
- Complex state machines
- Workflow-driven edits

Structure should emerge from use, not precede it.

---

## Error Handling and Recovery

Design intent:
- Fail safely
- Never lose data silently
- Prefer warnings over hard failures

If a task cannot be fully parsed, it should still be visible.

Partial usability is better than correctness that blocks access.

---

## Evolution and Change

ThreadKeeper is expected to evolve.

Design constraints:
- Avoid breaking existing data
- Prefer additive changes
- Keep migrations explicit and optional

Long-lived tools must respect their past.

---

## Summary

ThreadKeeper is designed to:
- Hold work in progress
- Preserve context
- Stay out of the way

It values:
- Simplicity over power
- Durability over novelty
- Human judgment over automation

If a feature makes the system smarter but the user dumber, it does not belong.

