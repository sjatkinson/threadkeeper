# ThreadKeeper

ThreadKeeper is a command-line task tracker. Tasks, notes, and attachments are stored as plain files in a directory on disk. There is no server, no database, and no sync.

A task has a subject, a body, a status, and zero or more tags. Notes can be attached to tasks as work progresses; each note is stored as a content-addressed blob. The data is meant to be readable with a shell, an editor, and `grep`.

ThreadKeeper is designed for one user. It does not track sprints, assignments, dependencies, or schedules. Tasks are expected to accumulate context, sit dormant, and sometimes be abandoned; the model assumes work is ongoing, not ticketed.

---


## Basic Usage Model

At a high level, you:
1. Create a task (a thread)
2. Add context as needed (notes, links, attachments)
3. Update its state as work progresses
4. Finish it, or let it go dormant

Tasks are meant to be lightweight and revisitable.  
You should feel comfortable creating tasks early and refining them later.

---

## Command Line Overview

ThreadKeeper is driven from the command line.

Typical interactions include:
- Creating a new task
- Listing current tasks
- Viewing a task and its context
- Adding notes or updates
- Marking tasks done, paused, or abandoned

Exact commands and flags are intentionally simple and discoverable via `--help`.

The CLI is designed to feel closer to tools like `git` or `todo.txt` than to a web application.

---

## Editing and Notes

ThreadKeeper favors **your editor**.

For longer descriptions or richer updates:
- Tasks can be edited using `$EDITOR`
- Notes are attached directly to tasks
- Bulk or structured edits are supported

Notes are meant to answer questions like:
- What was I thinking here?
- What did I try already?
- What’s blocking this?

---

## Data and Longevity

ThreadKeeper is built with long-term use in mind:
- Stable task identities
- Predictable storage
- No dependence on a running service

You should be able to come back to your data years later and still understand it.

---

## Status

ThreadKeeper is under active development and evolving through real use.

Design decisions are guided by:
- Daily friction
- Personal workflows
- What holds up over time

---

ThreadKeeper exists to keep hold of the threads that matter — long enough to finish them, or consciously let them go.
