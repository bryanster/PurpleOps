#!/usr/bin/env python3
"""An example Pydantic AI "red team" agent that records its attack steps in Blacklight.

The agent is given an adversary-emulation objective in natural language. It plans
the operation, then drives the Blacklight API through the generated Python SDK to:

  1. open a scenario in an engagement,
  2. register each ATT&CK-mapped attack step, and
  3. record what red actually executed against each step.

Blacklight is the source of truth for the assessment — this agent is just one way
to fill in the red side of the workbook. Everything the agent does goes through a
scoped service token, exactly as a human operator's automation would.

Nothing here executes an attack. "Executing a step" means *recording* in Blacklight
that a step was run — the command, the hosts, the outcome — so blue can score
detection against it. The agent narrates an emulation plan; the offensive actions
themselves are assumed to have happened (or to be happening) out of band on an
authorised range.

Run it
------
From ``sdk/python`` (so ``import blacklight`` resolves), with the SDK's deps plus
Pydantic AI installed::

    pip install pydantic-ai
    export ANTHROPIC_API_KEY=sk-ant-...
    export BLACKLIGHT_URL=https://blacklight.example.com
    export BLACKLIGHT_TOKEN=bl_<prefix>_<secret>      # a scoped service token
    # optional: pin an existing engagement instead of letting the agent create one
    export BLACKLIGHT_ENGAGEMENT_ID=<uuid>

    python examples/red_team_agent.py "Emulate an initial-access-to-exfil chain \
        against the finance VLAN using a phishing foothold."

The service token's principal must hold the lead/red role on the engagement — the
same authorisation a human red operator needs to write the workbook.
"""

from __future__ import annotations

import asyncio
import datetime as dt
import os
import sys
from dataclasses import dataclass, field
from uuid import UUID

from pydantic_ai import Agent, ModelRetry, RunContext

from blacklight.client import AuthenticatedClient
from blacklight.deployment import connect
from blacklight.types import UNSET
from blacklight.models import (
    CreateEngagement,
    CreateScenario,
    CreateStep,
    ExecutionStatus,
    Problem,
    RedExecutionPatch,
)

# Operations are modules under blacklight.api.<tag>.<operation_id>; import the
# async entry points so the agent's tools never block the event loop.
from blacklight.api.engagements.create_engagement import asyncio as create_engagement
from blacklight.api.engagements.list_engagements import asyncio as list_engagements
from blacklight.api.executions.list_engagement_executions import asyncio as list_executions
from blacklight.api.executions.patch_red_execution import asyncio as patch_red_execution
from blacklight.api.scenarios.create_scenario import asyncio as create_scenario
from blacklight.api.scenarios.create_step import asyncio as create_step


# --------------------------------------------------------------------------- #
# Dependencies: the live client and the engagement the agent is scoped to.
# Pydantic AI injects this into every tool via RunContext, so credentials and
# the target engagement never live in the model's context — only ids do.
# --------------------------------------------------------------------------- #
@dataclass
class RedTeamDeps:
    client: AuthenticatedClient
    engagement_id: UUID
    #: attackVersion pinned on the engagement — steps resolve techniques against it.
    attack_version: str
    #: Human-readable label for the operator this token acts as (audit trail only).
    operator: str = "pydantic-ai red agent"
    #: Scenario ids the agent has opened this run, newest last. Lets a tool
    #: default to "the scenario we're working in" when the model omits it.
    scenarios: list[str] = field(default_factory=list)


def _unwrap(result: object, what: str) -> object:
    """Turn the SDK's ``T | Problem | None`` union into a value or a ModelRetry.

    A documented failure comes back parsed as a :class:`Problem` (never raised);
    an undocumented status comes back as ``None``. Both are surfaced to the model
    as a retryable error so it can correct course (fix a technique id, pick a
    different scenario) rather than crashing the run.
    """
    if isinstance(result, Problem):
        raise ModelRetry(f"{what} was rejected by Blacklight [{result.code}]: {result.detail or result.title}")
    if result is None:
        raise ModelRetry(f"{what} returned an unexpected, undocumented response from Blacklight.")
    return result


agent = Agent(
    # The skill default: claude-opus-5. Pydantic AI reads ANTHROPIC_API_KEY itself.
    os.environ.get("RED_TEAM_MODEL", "anthropic:claude-opus-5"),
    deps_type=RedTeamDeps,
    system_prompt=(
        "You are a red-team operator running an authorised adversary-emulation "
        "engagement in Blacklight. Your job is to translate the objective you are "
        "given into an ordered chain of ATT&CK-mapped steps and to RECORD them in "
        "Blacklight so the blue team can score detection.\n\n"
        "Workflow, in order:\n"
        "1. Open one scenario for the operation with `open_scenario`, giving it a "
        "narrative and the threat actor you are emulating.\n"
        "2. For each attack step, call `register_attack_step` with a real ATT&CK "
        "technique id (e.g. T1566.001, T1059.001). Blacklight resolves the "
        "technique against the engagement's pinned version, so use canonical ids.\n"
        "3. Immediately `record_execution` for each step you registered, using the "
        "execution_id and version that `register_attack_step` returned — set the "
        "command you ran, the source/target hosts, and status `complete` (or "
        "`blocked` if a control stopped you).\n\n"
        "You are documenting, not attacking: never claim to have compromised a real "
        "system. Keep steps concrete and technically plausible. When done, give a "
        "short summary of the scenario and the steps you logged."
    ),
)


@agent.tool
async def open_scenario(
    ctx: RunContext[RedTeamDeps],
    name: str,
    threat_actor: str,
    narrative: str,
) -> str:
    """Open a scenario in the engagement to group this operation's steps.

    Returns the new scenario id, which you pass to `register_attack_step`.
    """
    scenario = _unwrap(
        await create_scenario(
            ctx.deps.engagement_id,
            client=ctx.deps.client,
            body=CreateScenario(name=name, threat_actor=threat_actor, narrative=narrative),
        ),
        "Creating the scenario",
    )
    sid = str(scenario.id)  # type: ignore[attr-defined]
    ctx.deps.scenarios.append(sid)
    return sid


@agent.tool
async def register_attack_step(
    ctx: RunContext[RedTeamDeps],
    scenario_id: str,
    name: str,
    technique_external_id: str,
    objective: str,
    target_asset: str = "",
    tools: list[str] | None = None,
) -> dict[str, str | int]:
    """Register one ATT&CK-mapped attack step in a scenario.

    `technique_external_id` is a canonical ATT&CK id such as "T1059.001"; Blacklight
    resolves it against the engagement's pinned version. Creating a step also opens a
    matching execution row (status `pending`) — this tool returns its `execution_id`
    and `version` so you can immediately `record_execution` against it.
    """
    step = _unwrap(
        await create_step(
            ctx.deps.engagement_id,
            UUID(scenario_id),
            client=ctx.deps.client,
            body=CreateStep(
                name=name,
                objective=objective,
                technique_external_id=technique_external_id,
                target_asset=target_asset,
                tools=tools or [],
            ),
        ),
        f"Registering step '{name}'",
    )
    step_id = step.id  # type: ignore[attr-defined]

    # The sibling execution is created in the same transaction; find it by step id.
    executions = _unwrap(
        await list_executions(ctx.deps.engagement_id, client=ctx.deps.client, scenario_id=UUID(scenario_id)),
        "Listing executions for the new step",
    )
    match = next((e for e in (executions.items or []) if e.step_id == step_id), None)  # type: ignore[attr-defined]
    if match is None:
        raise ModelRetry(f"Step '{name}' was created but its execution row was not found; retry the registration.")

    return {"step_id": str(step_id), "execution_id": str(match.id), "version": match.version}


@agent.tool
async def record_execution(
    ctx: RunContext[RedTeamDeps],
    execution_id: str,
    version: int,
    command_run: str,
    status: str = "complete",
    source_host: str = "",
    target_host: str = "",
    red_notes: str = "",
) -> dict[str, str | int]:
    """Record what red executed against a step's execution row.

    `version` is the value from `register_attack_step` (optimistic concurrency — a
    stale version is rejected and you should re-read and retry). `status` is one of
    pending/running/complete/blocked/skipped. Returns the execution's new version.
    """
    try:
        status_enum = ExecutionStatus(status)
    except ValueError:
        raise ModelRetry(
            f"'{status}' is not a valid status; use one of: {', '.join(s.value for s in ExecutionStatus)}."
        )

    now = dt.datetime.now(dt.timezone.utc)
    execution = _unwrap(
        await patch_red_execution(
            ctx.deps.engagement_id,
            UUID(execution_id),
            client=ctx.deps.client,
            body=RedExecutionPatch(
                version=version,
                status=status_enum,
                command_run=command_run,
                source_host=source_host,
                target_host=target_host,
                red_notes=red_notes,
                started_at=now,
                ended_at=now if status_enum in (ExecutionStatus.COMPLETE, ExecutionStatus.BLOCKED) else UNSET,
            ),
        ),
        "Recording the red execution",
    )
    return {"execution_id": execution_id, "new_version": execution.version, "status": execution.status.value}  # type: ignore[attr-defined]


# --------------------------------------------------------------------------- #
# Bootstrapping: resolve (or create) the engagement, then run the agent.
# --------------------------------------------------------------------------- #
async def resolve_engagement(client: AuthenticatedClient) -> tuple[UUID, str]:
    """Use the pinned engagement if given, else create a demo one to write into."""
    pinned = os.environ.get("BLACKLIGHT_ENGAGEMENT_ID")
    if pinned:
        # Find it in the list so we can read back its pinned ATT&CK version.
        page = _unwrap(await list_engagements(client=client, limit=100), "Listing engagements")
        for eng in page.items or []:  # type: ignore[attr-defined]
            if str(eng.id) == pinned:
                return eng.id, eng.attack_version
        raise SystemExit(f"Engagement {pinned} not found or not visible to this token.")

    created = _unwrap(
        await create_engagement(
            client=client,
            body=CreateEngagement(
                name=f"Pydantic AI red-agent demo {dt.date.today().isoformat()}",
                attack_version="17.1",
                description="Auto-created by the example red_team_agent.py.",
            ),
        ),
        "Creating the demo engagement",
    )
    return created.id, created.attack_version  # type: ignore[attr-defined]


async def main() -> None:
    url = os.environ.get("BLACKLIGHT_URL")
    token = os.environ.get("BLACKLIGHT_TOKEN")
    if not url or not token:
        raise SystemExit("Set BLACKLIGHT_URL and BLACKLIGHT_TOKEN (a scoped service token).")

    objective = " ".join(sys.argv[1:]) or (
        "Emulate a phishing-to-exfiltration chain against a Windows workstation on "
        "the corporate VLAN. Cover initial access, execution, credential access, and "
        "exfiltration."
    )

    client = connect(url, token=token, raise_on_unexpected_status=False)
    async with client as client:  # AuthenticatedClient is an async context manager
        engagement_id, attack_version = await resolve_engagement(client)
        deps = RedTeamDeps(client=client, engagement_id=engagement_id, attack_version=attack_version)

        print(f"→ Engagement {engagement_id} (ATT&CK {attack_version})")
        print(f"→ Objective: {objective}\n")

        result = await agent.run(objective, deps=deps)
        print(result.output)
        print(f"\n✓ Logged {len(deps.scenarios)} scenario(s); open the engagement in Blacklight to score detection.")


if __name__ == "__main__":
    asyncio.run(main())
