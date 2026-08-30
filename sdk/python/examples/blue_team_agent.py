#!/usr/bin/env python3
"""An example Pydantic AI "blue team" agent that scores detection in Blacklight.

The counterpart to ``red_team_agent.py``. Where the red agent *records* what was
executed, this agent reads those executions back and fills in the blue side of the
same workbook:

  1. review the steps red logged in an engagement,
  2. score detection on each execution — category, protection, MTTD, the alerting
     source and rule — and
  3. open findings for the gaps (missed or slow detections) so blue can remediate.

Blacklight is the source of truth; this agent is one way to fill in the blue side.
Everything goes through a scoped service token, exactly as a defender's automation
would.

Nothing here inspects a real SIEM. "Scoring detection" means recording in Blacklight
*whether and how* a recorded attack step was detected or prevented. In a real
deployment you would replace the reasoning with lookups against your EDR/SIEM (or
feed the agent those results); here the agent reasons over the step's ATT&CK context
to produce a plausible, well-formed assessment.

Run it
------
From ``sdk/python`` (so ``import blacklight`` resolves), with the SDK's deps plus
Pydantic AI installed::

    pip install pydantic-ai
    export ANTHROPIC_API_KEY=sk-ant-...
    export BLACKLIGHT_URL=https://blacklight.example.com
    export BLACKLIGHT_TOKEN=bl_<prefix>_<secret>      # a scoped service token
    export BLACKLIGHT_ENGAGEMENT_ID=<uuid>            # the engagement to score

    python examples/blue_team_agent.py

The service token's principal must hold the lead/blue role on the engagement — the
same authorisation a human defender needs to score the workbook and file findings.
"""

from __future__ import annotations

import asyncio
import datetime as dt
import os
import sys
from dataclasses import dataclass
from uuid import UUID

from pydantic_ai import Agent, ModelRetry, RunContext

from blacklight.client import AuthenticatedClient
from blacklight.deployment import connect
from blacklight.types import UNSET
from blacklight.models import (
    AlertSeverity,
    BlueDetectionPatch,
    DetectionCategory,
    FindingSeverity,
    NewFinding,
    Problem,
    Protection,
)

# Operations are modules under blacklight.api.<tag>.<operation_id>; import the
# async entry points so the agent's tools never block the event loop.
from blacklight.api.engagements.create_finding import asyncio as create_finding
from blacklight.api.executions.list_engagement_executions import asyncio as list_executions
from blacklight.api.executions.patch_blue_detection import asyncio as patch_blue_detection
from blacklight.api.scenarios.list_engagement_steps import asyncio as list_steps


# --------------------------------------------------------------------------- #
# Dependencies injected into every tool via RunContext, so credentials and the
# target engagement stay in the process — only execution ids reach the model.
# --------------------------------------------------------------------------- #
@dataclass
class BlueTeamDeps:
    client: AuthenticatedClient
    engagement_id: UUID
    #: Human-readable label for the operator this token acts as (audit trail only).
    analyst: str = "pydantic-ai blue agent"


def _unwrap(result: object, what: str) -> object:
    """Turn the SDK's ``T | Problem | None`` union into a value or a ModelRetry.

    Documented failures come back parsed as a :class:`Problem` (never raised); an
    undocumented status comes back as ``None``. Both are surfaced to the model as a
    retryable error so it can correct course (a stale version, a bad enum) rather
    than crashing the run.
    """
    if isinstance(result, Problem):
        raise ModelRetry(f"{what} was rejected by Blacklight [{result.code}]: {result.detail or result.title}")
    if result is None:
        raise ModelRetry(f"{what} returned an unexpected, undocumented response from Blacklight.")
    return result


agent = Agent(
    os.environ.get("BLUE_TEAM_MODEL", "anthropic:claude-opus-5"),
    deps_type=BlueTeamDeps,
    system_prompt=(
        "You are a blue-team analyst scoring detection on an authorised adversary-"
        "emulation engagement in Blacklight. Red has already recorded the steps they "
        "executed; your job is to assess each one and RECORD your detection outcome, "
        "then open findings for the gaps.\n\n"
        "Workflow:\n"
        "1. Call `review_recorded_steps` to see the steps red logged, each with its "
        "ATT&CK technique and the execution_id + version you will score against.\n"
        "2. For each execution, call `score_detection`. Judge, from the technique and "
        "the command red ran, how a well-instrumented SOC would have fared: set the "
        "detection category (none/telemetry/general/tactic/technique), protection "
        "(blocked/partial/not_blocked/n/a), the detecting source and rule when "
        "detected, an alert severity, and detected_at (drives mean-time-to-detect). "
        "Be realistic — assume gaps exist; do not mark everything as detected.\n"
        "3. For any step with weak or missing detection (category none/telemetry, or "
        "not blocked on a high-impact technique), call `open_finding` linked to that "
        "execution so blue can remediate before retest.\n\n"
        "You are assessing telemetry, not attacking. When done, summarise coverage: "
        "how many steps were detected vs. missed, and the findings you opened."
    ),
)


@agent.tool
async def review_recorded_steps(ctx: RunContext[BlueTeamDeps]) -> list[dict[str, object]]:
    """List the steps red recorded, joined to their execution rows.

    Returns one entry per execution with the ATT&CK technique, red's command and
    notes, the current detection state, and the `execution_id` + `version` you pass
    to `score_detection` (version is optimistic-concurrency — always use the value
    from here).
    """
    steps = _unwrap(
        await list_steps(ctx.deps.engagement_id, client=ctx.deps.client),
        "Listing steps",
    )
    executions = _unwrap(
        await list_executions(ctx.deps.engagement_id, client=ctx.deps.client),
        "Listing executions",
    )
    by_step = {s.id: s for s in (steps.items or [])}  # type: ignore[attr-defined]

    rows: list[dict[str, object]] = []
    for e in executions.items or []:  # type: ignore[attr-defined]
        step = by_step.get(e.step_id)
        rows.append(
            {
                "execution_id": str(e.id),
                "version": e.version,
                "status": e.status.value,
                "step_name": getattr(step, "name", "") if step else "",
                "technique_id": (getattr(step, "technique_id", "") or getattr(step, "subtechnique_id", "") or "") if step else "",
                "objective": getattr(step, "objective", "") if step else "",
                "command_run": getattr(e, "command_run", "") or "",
                "red_notes": getattr(e, "red_notes", "") or "",
                "current_outcome": e.outcome.value if getattr(e, "outcome", None) not in (None, UNSET) else "unscored",
            }
        )
    if not rows:
        raise ModelRetry("No executions found in this engagement — red has not recorded any steps yet.")
    return rows


@agent.tool
async def score_detection(
    ctx: RunContext[BlueTeamDeps],
    execution_id: str,
    version: int,
    detection_category: str,
    protection: str = "n/a",
    detecting_source: str = "",
    detecting_rule_ref: str = "",
    alert_severity: str = "",
    detected_at_iso: str = "",
    blue_notes: str = "",
) -> dict[str, object]:
    """Record blue's detection outcome for one execution.

    `detection_category` is none/telemetry/general/tactic/technique; `protection` is
    blocked/partial/not_blocked/n/a. `detected_at_iso` (ISO-8601) is when the alert
    fired — Blacklight derives MTTD from it, so set it whenever the category is not
    `none`. `version` comes from `review_recorded_steps`. Returns the resulting
    outcome and new version.
    """
    try:
        category = DetectionCategory(detection_category)
        prot = Protection(protection)
    except ValueError as exc:
        raise ModelRetry(
            f"Invalid enum value ({exc}). "
            f"detection_category ∈ {{{', '.join(c.value for c in DetectionCategory)}}}; "
            f"protection ∈ {{{', '.join(p.value for p in Protection)}}}."
        )

    detected_at: dt.datetime | object = UNSET
    if detected_at_iso:
        try:
            detected_at = dt.datetime.fromisoformat(detected_at_iso)
        except ValueError:
            raise ModelRetry(f"detected_at_iso '{detected_at_iso}' is not ISO-8601 (e.g. 2026-08-29T14:05:00+00:00).")
    elif category is not DetectionCategory.NONE:
        detected_at = dt.datetime.now(dt.timezone.utc)

    severity: AlertSeverity | object = UNSET
    if alert_severity:
        try:
            severity = AlertSeverity(alert_severity)
        except ValueError:
            raise ModelRetry(
                f"alert_severity '{alert_severity}' invalid; use one of: "
                f"{', '.join(s.value for s in AlertSeverity)}."
            )

    execution = _unwrap(
        await patch_blue_detection(
            ctx.deps.engagement_id,
            UUID(execution_id),
            client=ctx.deps.client,
            body=BlueDetectionPatch(
                version=version,
                detection_category=category,
                protection=prot,
                detected_at=detected_at,  # type: ignore[arg-type]
                detecting_source=detecting_source,
                detecting_rule_ref=detecting_rule_ref,
                alert_severity=severity,  # type: ignore[arg-type]
                blue_notes=blue_notes,
            ),
        ),
        "Scoring detection",
    )
    outcome = getattr(execution, "outcome", None)
    mttd = getattr(execution, "mttd_seconds", None)
    return {
        "execution_id": execution_id,
        "new_version": execution.version,  # type: ignore[attr-defined]
        "outcome": outcome.value if outcome not in (None, UNSET) else "n/a",
        "mttd_seconds": mttd if mttd not in (None, UNSET) else None,
    }


@agent.tool
async def open_finding(
    ctx: RunContext[BlueTeamDeps],
    title: str,
    description: str,
    severity: str,
    created_from_execution: str,
    recommendation: str = "",
) -> str:
    """Open a finding for a detection gap, linked to the execution that revealed it.

    `severity` is info/low/medium/high/critical. `created_from_execution` is the
    execution_id from `review_recorded_steps`. Returns the new finding id.
    """
    try:
        sev = FindingSeverity(severity)
    except ValueError:
        raise ModelRetry(
            f"severity '{severity}' invalid; use one of: {', '.join(s.value for s in FindingSeverity)}."
        )
    finding = _unwrap(
        await create_finding(
            ctx.deps.engagement_id,
            client=ctx.deps.client,
            body=NewFinding(
                title=title,
                description=description,
                severity=sev,
                recommendation=recommendation,
                created_from_execution=UUID(created_from_execution),
            ),
        ),
        f"Opening finding '{title}'",
    )
    return str(finding.id)  # type: ignore[attr-defined]


# --------------------------------------------------------------------------- #
# Bootstrapping: the blue agent scores an existing engagement, so the id is
# required — unlike the red agent, it does not create work out of nothing.
# --------------------------------------------------------------------------- #
async def main() -> None:
    url = os.environ.get("BLACKLIGHT_URL")
    token = os.environ.get("BLACKLIGHT_TOKEN")
    raw_id = os.environ.get("BLACKLIGHT_ENGAGEMENT_ID")
    if not url or not token:
        raise SystemExit("Set BLACKLIGHT_URL and BLACKLIGHT_TOKEN (a scoped service token).")
    if not raw_id:
        raise SystemExit("Set BLACKLIGHT_ENGAGEMENT_ID to the engagement to score (blue scores existing work).")
    engagement_id = UUID(raw_id)

    instruction = " ".join(sys.argv[1:]) or (
        "Review every step red recorded in this engagement, score detection on each "
        "one against a realistic SOC posture, and open findings for the gaps."
    )

    client = connect(url, token=token, raise_on_unexpected_status=False)
    async with client as client:  # AuthenticatedClient is an async context manager
        deps = BlueTeamDeps(client=client, engagement_id=engagement_id)

        print(f"→ Engagement {engagement_id}")
        print(f"→ Task: {instruction}\n")

        result = await agent.run(instruction, deps=deps)
        print(result.output)
        print("\n✓ Scored in Blacklight — open the engagement to see coverage, MTTD, and the findings.")


if __name__ == "__main__":
    asyncio.run(main())
