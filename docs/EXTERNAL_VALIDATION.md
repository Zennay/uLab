# External validation protocol

This protocol is for evaluating uLab with a real maintainer, contributor, or release engineer on an actual upgrade path. It is intentionally neutral: a failed integration, a preference for existing scripts, or a decision not to use uLab is valid product evidence.

## What this protocol is trying to learn

The current questions are:

1. Can a real project express its setup, upgrade, and verification semantics through project-owned hooks without changing uLab core code?
2. Does uLab remove useful orchestration work such as version-matrix execution, isolation, cleanup, evidence capture, and release gating?
3. Is the resulting evidence understandable and trustworthy enough for release work?
4. What onboarding friction or missing capability would stop continued use?

This is not a benchmark and it should not be used to manufacture an adoption claim.

## Participant and project

Record:

- participant role and relationship to the project;
- public project/repository when it can be shared;
- existing upgrade-validation workflow before uLab;
- relevant runner, state model, source versions, and target version.

The participant should have enough project context to judge whether an upgrade is valid. A hypothetical project or AI-generated evaluation does not count as external maintainer evidence.

## Before starting the uLab integration

Capture the current workflow before showing or discussing a preferred uLab solution:

- commands or CI jobs currently used;
- manual coordination steps;
- how multiple source versions are handled;
- how cleanup/isolation is handled;
- what evidence is kept after a run;
- how a failed path blocks a release;
- known pain points in the existing workflow.

Do not convert this baseline into a numeric "time saved" estimate unless both sides are actually measured or the participant clearly labels an estimate.

## Integration task

Use a disposable development or test environment. Never test a destructive upgrade against production data.

Ask the participant to integrate one real upgrade path while keeping application-specific correctness in project-owned commands or assertions.

The target outcome is a real uLab run that contains:

- at least one historical source version;
- one target version;
- project-owned setup/upgrade/verify hooks;
- persistent evidence;
- cleanup;
- a release-gate result.

If the project already has a natural failing upgrade or assertion, capture it. Do not invent a destructive production scenario just to create a failure.

## Measurements

Record the following as observed values where possible:

- start time and first meaningful successful or failing run time;
- approximate number of meaningful manual setup/run/inspection steps;
- number and purpose of project-owned hook/config files added;
- any uLab core modification required;
- runtime of the final reproducible test;
- failures encountered during onboarding and how they were resolved.

For Docker-backed integrations, keep image digests and Docker/Compose versions with the evidence when practical.

## Evidence to retain

Keep enough evidence to reproduce the conclusion:

- exact uLab version or Git commit;
- project/version matrix;
- relevant uLab config;
- evidence bundle metadata and result;
- reproduction command;
- environment/toolchain metadata when relevant;
- notes about any workaround or core patch.

Remove secrets, tokens, credentials, private application data, and other sensitive output before sharing evidence publicly.

## Debrief questions

Ask these after the participant has completed the task. Do not answer them on the participant's behalf.

1. What did uLab make easier, if anything?
2. What remained project-specific?
3. What was the biggest source of friction?
4. Was any uLab behavior surprising or hard to trust?
5. Which part of the generated evidence would you actually use during a release decision?
6. What would you remove or simplify?
7. Is there anything you expected uLab to own that it deliberately leaves to project hooks?
8. Based on this attempt, would you use uLab again for a real upgrade path? Why or why not?

Capture the response through the **External integration feedback** GitHub issue form when the participant is comfortable sharing it publicly.

## Interpreting the result

Treat one integration as a case, not a universal conclusion.

A result supports the current orchestration boundary when:

- the project can integrate without project-specific changes to uLab core;
- the participant can identify orchestration work uLab replaced or simplified;
- the evidence/release gate is useful in the participant's real workflow.

A result is evidence against the current approach when, for example:

- onboarding costs more than the project's existing solution without a compensating benefit;
- required semantics repeatedly leak into uLab core;
- the participant cannot trust or use the evidence;
- the abstraction hides information needed for release decisions.

Record contradictory results instead of averaging them away. Architecture changes should follow repeated evidence across more than one project, not a single request.

## M3 evidence boundary

For the first external integration, do not mark the milestone fully verified merely because:

- an integration exists in the uLab repository;
- the scripted runtime proof passes;
- an AI review considers the design reasonable;
- line count appears small.

Those facts can verify implementation and runtime behavior. External usability/value claims require an actual participant and their observed workflow.
