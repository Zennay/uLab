# M5 maintainer usability validation

This protocol validates the visual diagnosis surface only. It does not validate adoption, onboarding effort, time saved, or the usefulness of uLab's integration model.

A qualifying session uses a real maintainer, regular contributor, or release engineer. Do not substitute an AI evaluation, the project owner reviewing their own UI, or a generic consumer participant.

## Evidence boundary

The M5 question is narrow:

> Can a maintainer or release engineer understand a release verdict, locate the broken upgrade path, identify the recorded failure, find the evidence, and recover the reproduction command without an explanation of uLab internals?

The deterministic session fixture is intentionally synthetic. It exists so every participant sees the same evidence and the UI itself is what changes the outcome.

Because the repository does not yet have a selected project license, the lowest-friction session is facilitator-operated: the facilitator prepares and serves the UI and the participant interacts with the browser. Do not require an external participant to clone, modify, or redistribute the repository unless the project owner has separately cleared that permission.

## Participant

Record before the session:

- role;
- relevant maintainer/release responsibility;
- familiarity with CI, migrations, or upgrade validation;
- whether the participant has seen uLab before.

Do not explain the setup/upgrade/verify architecture before the tasks. A one-sentence neutral framing is enough:

> This screen shows evidence from automated software upgrade checks. Use what is visible to answer the release questions.

## Facilitator setup

Use a clean checkout of the exact revision being evaluated.

Prepare one deterministic evidence bundle:

```sh
sh scripts/prepare-m5-usability.sh
```

The script prints an exact command for starting the read-only UI. Run that command, then open:

```text
http://127.0.0.1:8080/
```

The participant should see the browser, not the preparation script, terminal output, or this answer key.

For a second session, pass a fresh output directory rather than overwriting prior evidence:

```sh
sh scripts/prepare-m5-usability.sh .ulab/m5-usability-session-2
```

## Participant tasks

Ask these in order. Do not point at controls or explain uLab terminology unless the participant explicitly gives up; record every hint.

1. **Release verdict** — "Based on this screen, would this tested release pass its configured release gate? Show me what tells you that."
2. **Broken path** — "Which source-version upgrade path needs attention?"
3. **Failure diagnosis** — "What kind of failure was recorded, and in which phase did it occur?"
4. **Evidence and reproduction** — "Show me the evidence you would inspect next, then find the exact command or information needed to reproduce this run."
5. **Scope of proof** — "What did this result actually test, and what would you *not* conclude from it?"

After the five tasks, ask only:

- What was hardest to find?
- Was any label or status ambiguous?
- What information did you expect but not see?
- Would you trust this screen enough to decide what to investigate next? Why or why not?

## What to record

For each task record:

| Field | Value |
| --- | --- |
| task | 1–5 |
| outcome | correct / incorrect / gave up |
| time to answer | observed duration |
| hints | exact count and wording |
| navigation | what the participant clicked/opened |
| participant wording | short verbatim note |
| confusion | observed issue, if any |

Also retain:

- participant role;
- session date;
- exact uLab commit printed by the preparation script;
- browser/UI revision;
- session evidence directory;
- facilitator name;
- any screen recording or notes only with participant permission.

Do not convert hesitation, line count, or facilitator impressions into a "time saved" claim.

## Recording the result

After the session, use the repository's **M5 usability session result** issue form. Copy observations while they are still fresh and keep the record literal: include wrong answers, hints, confusion, and negative feedback.

The issue form is for M5 visual-usability evidence only. Do not use it to claim adoption, onboarding speed, orchestration value, or toil reduction. Those require the separate external-integration protocol.

## Facilitator answer key

The prepared fixture intentionally contains one non-passing path.

Expected observations:

- release readiness is **BLOCKED** because the configured policy requires all paths;
- 2 of 3 paths pass;
- `v1.9.0 -> v3.0.0` is the non-passing path;
- the failure is recorded as a **project hook** failure in **verify**;
- phase evidence exposes the recorded command/output/error;
- provenance/reproduction exposes the persisted bundle identity and reproduction command.

For task 5, the important boundary is conceptual rather than exact wording: the screen proves the configured upgrade paths and project-defined assertions that were actually executed. It does not prove universal application correctness, untested versions, or semantics the project did not assert.

## M5 exit decision

Do not award maintainer-usability credit merely because the fixture runs.

The current M5 usability gate can move only after at least one qualifying real participant attempts all five tasks. Record the outcome even if the session fails.

A passing session requires the participant to complete the five diagnosis tasks without an explanation of uLab's internal architecture. Navigation mistakes, hints, wrong answers, ambiguity, or distrust are product evidence and must remain visible in the record rather than being edited away.

This UI-only session can support the **maintainer usability validation** criterion and the visual-evidence hypothesis. It does not close the separate external-integration/onboarding or measured-toil criteria.
