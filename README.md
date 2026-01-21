# ThreadKeeper

ThreadKeeper is a local task manager for people with lots of ongoing **threads** of work. 

It is designed for individuals (and very small teams) who want a fast, durable, text-centric way to track work without turning their lives into a project management system.

This is not Jira.  
This is not a second brain.  
This is a place to keep track of what you are doing — and why.

---

## Philosophy

ThreadKeeper is built around a few core ideas.

### Tasks are threads of attention

A task is not just a checkbox.  
It is a thread: something you pick up, put down, return to, and occasionally abandon.

ThreadKeeper assumes:
- Tasks evolve over time
- Notes and context matter
- You often need to remember *where you left off*, not just *that something exists*

### Human scale, not enterprise scale

ThreadKeeper is intentionally scoped:
- Single user by default
- Possibly shared across a few machines
- Maybe usable by a very small team

There are no sprints, story points, burndown charts, or workflow engines.  

### Local-first and durable

Your data lives on your machine. It is:
- Fast
- Inspectable
- Back-uppable
- Under your control

ThreadKeeper is designed to work well with:
- Version control
- Backups
- Long-lived personal archives

### Low ceremony, low friction

Creating or updating a task should be easier than avoiding it.

ThreadKeeper prefers:
- Simple commands
- Optional structure
- Text over forms
- Your editor over custom UIs


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
