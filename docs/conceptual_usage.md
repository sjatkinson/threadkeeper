# ThreadKeeper Usage

This document describes how ThreadKeeper is intended to be used day to day.

It is not a full command reference.
Exact flags and subcommands may evolve, but the workflow and expectations should remain stable.

---

## Mental Model

ThreadKeeper manages **threads**, not just tasks.

A thread represents something you are paying attention to:
- A piece of work
- A problem you are thinking through
- Something you intend to return to

Threads are cheap to create and safe to abandon.
They are meant to reflect how work actually unfolds.

---

## Creating Threads

You typically create a thread as soon as something starts pulling at your attention.

Creation should feel lightweight:
- A short title
- Optional initial description
- Optional tags

You do **not** need to fully understand the work yet.
Threads are allowed to start vague and become clearer over time.

---

## Listing and Reviewing

Most interactions with ThreadKeeper begin by reviewing existing threads.

Typical review patterns:
- What am I currently working on?
- What has gone stale?
- What did I leave unfinished?

Listing commands are expected to be fast and readable, favoring:
- Concise output
- Meaningful defaults
- Filtering over sorting gymnastics

ThreadKeeper is designed to be checked often.

---

## Viewing a Thread

Viewing a thread shows:
- Its title and identity
- Current state
- Accumulated notes and updates
- Tags and references

A thread should answer:
- What is this?
- Why does it exist?
- Where did I leave off?

If those answers are missing, the thread needs more context — not more structure.

---

## Adding Notes and Updates

Notes are how threads grow.

Use notes to capture:
- Partial progress
- Ideas and dead ends
- Decisions made
- Questions to revisit

Notes are append-only by default.
They form a lightweight history of your thinking, not a polished narrative.

Short updates are common.
Long updates should open in your editor.

---

## Editing Threads

Threads can be edited at any time.

Editing is intended for:
- Refining titles or descriptions
- Cleaning up context
- Correcting mistakes

For anything non-trivial, ThreadKeeper prefers invoking `$EDITOR`.

Bulk edits are supported when you need to reshape several threads at once.

---

## States and Completion

Threads typically move through loose states such as:
- Active
- Paused
- Done
- Abandoned

These states are intentionally simple.

A thread does not need to be “completed” to be useful.
Abandoned threads still carry context and may be worth revisiting later.

ThreadKeeper does not judge unfinished work.

---

## Tags and Organization

Tags provide lightweight organization.

Use tags to:
- Group related threads
- Mark areas of responsibility
- Create ad-hoc views

Tags are:
- Optional
- Free-form
- Non-hierarchical

Inconsistency is tolerated.
Perfect taxonomy is not required.

---

## Attachments and References

Threads may reference:
- Files
- URLs
- External documents

Attachments exist to support work, not to centralize everything.

ThreadKeeper does not require you to move your files or rewrite your workflow.

---

## Daily Use Patterns

Common usage patterns include:
- Creating threads early and often
- Adding short notes during or after work
- Reviewing threads at the start or end of the day
- Letting inactive threads sit without guilt

ThreadKeeper works best when it reflects reality, not aspiration.

---

## What Not to Do

ThreadKeeper is not well-suited for:
- Breaking work into tiny tasks
- Tracking hours or productivity metrics
- Enforcing schedules or deadlines
- Maintaining exhaustive documentation

If you feel pressure to “keep ThreadKeeper tidy,” you are probably overusing it.

---

## Closing Thought

ThreadKeeper is a memory aid, not a manager.

It exists to help you hold onto the threads of your work long enough to:
- Make progress
- Capture insight
- Or consciously let go

Use it lightly. Let it earn its place.
