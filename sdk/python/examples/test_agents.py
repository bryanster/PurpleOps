"""End-to-end tests for the example red- and blue-team agents.

These prove the agents *can actually work*: that the tools are wired correctly and
that a tool call flows all the way through the generated SDK to an HTTP request and
back into a parsed model. Two things are faked, and only those two:

* **The model** — a Pydantic AI ``FunctionModel`` scripts the exact tool calls, so
  no ``ANTHROPIC_API_KEY`` and no LLM are involved. This is the model's job (decide
  which tool to call with which arguments); we pin it so the test is deterministic.
* **Blacklight** — ``respx`` intercepts httpx and answers each endpoint with a real
  SDK model serialized to its wire JSON, so the request-building, URL, body, and
  response-parsing are all exercised for real.

What is *not* faked: the agent, its tools, the SDK operations, and the whole
model→tool→SDK→HTTP→parse→tool-return loop.

Run from ``sdk/python``::

    pytest examples/
"""

from __future__ import annotations

import datetime as dt
import json
import re
from typing import Any
from uuid import UUID

import httpx
import pytest
import respx

import blue_team_agent
import red_team_agent
from blacklight.deployment import API_PATH, connect
from blacklight.models import (
    Execution,
    ExecutionList,
    ExecutionOutcome,
    ExecutionStatus,
    Finding,
    FindingSeverity,
    FindingStatus,
    Problem,
    ProblemCode,
    Scenario,
    ScenarioSource,
    Step,
    StepList,
)
from pydantic_ai import ModelRetry
from pydantic_ai.messages import (
    ModelResponse,
    TextPart,
    ToolCallPart,
    ToolReturnPart,
)
from pydantic_ai.models.function import AgentInfo, FunctionModel

ORIGIN = "https://blacklight.example.com"
BASE = f"{ORIGIN}{API_PATH}"
EID = "00000000-0000-0000-0000-0000000000e1"
SID = "11111111-1111-1111-1111-111111111111"
STEPID = "22222222-2222-2222-2222-222222222222"
EXECID = "33333333-3333-3333-3333-333333333333"
FINDID = "44444444-4444-4444-4444-444444444444"

_NOW = dt.datetime(2026, 8, 29, 12, 0, 0, tzinfo=dt.timezone.utc)
_TODAY = dt.date(2026, 8, 29)


# --------------------------------------------------------------------------- #
# Fake-response factories: build a real SDK model, serialize it to wire JSON.
# Using the model's own to_dict keeps the fake server honest — if a field the
# agent reads is dropped by the generator, these round-trips break.
# --------------------------------------------------------------------------- #
def _scenario() -> dict[str, Any]:
    return Scenario(
        id=UUID(SID), engagement_id=UUID(EID), ordinal=1, name="Phish to exfil",
        narrative="Emulate a phishing foothold to data exfil.", source=ScenarioSource.MANUAL,
        created_at=_NOW, updated_at=_NOW, threat_actor="FIN-EXAMPLE",
    ).to_dict()


def _step() -> dict[str, Any]:
    return Step(
        id=UUID(STEPID), scenario_id=UUID(SID), ordinal=1, name="Spearphishing Attachment",
        objective="Gain initial access", template_id="", target_asset="WKSTN-01",
        attack_version="17.1", created_at=_NOW, updated_at=_NOW, technique_id="T1566.001",
    ).to_dict()


def _execution(version: int, status: ExecutionStatus, **over: Any) -> dict[str, Any]:
    base = dict(
        id=UUID(EXECID), step_id=UUID(STEPID), version=version, status=status,
        executed_by="red@example.com", command_run="", source_host="", target_host="",
        red_notes="", detection_modifiers=[], detecting_source="", detecting_rule_ref="",
        alert_severity="", blue_notes="", scored_by="", created_at=_NOW, updated_at=_NOW,
    )
    base.update(over)
    return Execution(**base).to_dict()


def _finding() -> dict[str, Any]:
    return Finding(
        id=UUID(FINDID), engagement_id=UUID(EID), title="Missed spearphishing detection",
        description="No alert on T1566.001.", severity=FindingSeverity.HIGH, recommendation="Tune EDR.",
        owner="", status=FindingStatus.OPEN, created_at=_NOW, updated_at=_NOW, step_ids=[UUID(STEPID)],
    ).to_dict()


def _last_return(messages: list, tool: str) -> Any:
    """The content a given tool most recently returned, for threading ids forward."""
    for msg in reversed(messages):
        for part in getattr(msg, "parts", []):
            if isinstance(part, ToolReturnPart) and part.tool_name == tool:
                content = part.content
                if isinstance(content, str):
                    # dict/list tool returns arrive as Python objects; a plain
                    # string (e.g. a scenario id) arrives as-is, not JSON.
                    try:
                        return json.loads(content)
                    except json.JSONDecodeError:
                        return content
                return content
    raise AssertionError(f"no tool return found for {tool}")


def _responses_so_far(messages: list) -> int:
    return sum(1 for msg in messages if isinstance(msg, ModelResponse))


# --------------------------------------------------------------------------- #
# Red agent: open a scenario, register an ATT&CK step, record the execution.
# --------------------------------------------------------------------------- #
def _red_script(messages: list, info: AgentInfo) -> ModelResponse:
    n = _responses_so_far(messages)
    if n == 0:
        return ModelResponse(parts=[ToolCallPart("open_scenario", {
            "name": "Phish to exfil", "threat_actor": "FIN-EXAMPLE",
            "narrative": "Emulate a phishing foothold to data exfil.",
        })])
    if n == 1:
        sid = _last_return(messages, "open_scenario")
        return ModelResponse(parts=[ToolCallPart("register_attack_step", {
            "scenario_id": sid, "name": "Spearphishing Attachment",
            "technique_external_id": "T1566.001", "objective": "Gain initial access",
            "target_asset": "WKSTN-01", "tools": ["gophish"],
        })])
    if n == 2:
        reg = _last_return(messages, "register_attack_step")
        return ModelResponse(parts=[ToolCallPart("record_execution", {
            "execution_id": reg["execution_id"], "version": reg["version"],
            "command_run": "delivered weaponised .docx", "status": "complete",
            "source_host": "attacker-vps", "target_host": "WKSTN-01",
            "red_notes": "user opened the attachment",
        })])
    return ModelResponse(parts=[TextPart("Logged 1 scenario and 1 step.")])


def test_red_agent_records_an_attack_step_end_to_end() -> None:
    with respx.mock:
        scenarios = respx.post(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/scenarios$").mock(
            return_value=httpx.Response(201, json=_scenario())
        )
        steps = respx.post(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/scenarios/{SID}/steps$").mock(
            return_value=httpx.Response(201, json=_step())
        )
        list_exec = respx.get(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/executions(\?.*)?$").mock(
            return_value=httpx.Response(200, json=ExecutionList(
                items=[Execution.from_dict(_execution(0, ExecutionStatus.PENDING))]
            ).to_dict())
        )
        patch_exec = respx.patch(
            url__regex=rf"{re.escape(BASE)}/engagements/{EID}/executions/{EXECID}/execution$"
        ).mock(return_value=httpx.Response(200, json=_execution(1, ExecutionStatus.COMPLETE, command_run="delivered weaponised .docx")))

        deps = red_team_agent.RedTeamDeps(
            client=connect(ORIGIN, token="bl_test_secret"),
            engagement_id=UUID(EID), attack_version="17.1",
        )
        with red_team_agent.agent.override(model=FunctionModel(_red_script)):
            result = red_team_agent.agent.run_sync("Emulate a phishing-to-exfil chain.", deps=deps)

    # Every stage of the red workflow reached Blacklight.
    assert scenarios.called and steps.called and list_exec.called and patch_exec.called

    # The step carried a canonical ATT&CK id for server-side resolution.
    assert json.loads(steps.calls.last.request.content)["techniqueExternalId"] == "T1566.001"

    # The execution was patched with the pending row's version (optimistic concurrency)
    # and the command red ran.
    patch_body = json.loads(patch_exec.calls.last.request.content)
    assert patch_body["version"] == 0
    assert patch_body["status"] == "complete"
    assert patch_body["commandRun"] == "delivered weaponised .docx"

    # The agent finished and threaded the scenario id through its deps.
    assert "Logged" in result.output
    assert deps.scenarios == [SID]


# --------------------------------------------------------------------------- #
# Blue agent: review the recorded step, score it as a miss, open a finding.
# --------------------------------------------------------------------------- #
def _blue_script(messages: list, info: AgentInfo) -> ModelResponse:
    n = _responses_so_far(messages)
    if n == 0:
        return ModelResponse(parts=[ToolCallPart("review_recorded_steps", {})])
    if n == 1:
        rows = _last_return(messages, "review_recorded_steps")
        row = rows[0]
        return ModelResponse(parts=[ToolCallPart("score_detection", {
            "execution_id": row["execution_id"], "version": row["version"],
            "detection_category": "none", "protection": "not_blocked",
            "blue_notes": "No alert fired for the spearphishing attachment.",
        })])
    if n == 2:
        rows = _last_return(messages, "review_recorded_steps")
        row = rows[0]
        return ModelResponse(parts=[ToolCallPart("open_finding", {
            "title": "Missed spearphishing detection",
            "description": "T1566.001 executed with no telemetry or alert.",
            "severity": "high", "created_from_execution": row["execution_id"],
            "recommendation": "Add an EDR rule for Office spawning script interpreters.",
        })])
    return ModelResponse(parts=[TextPart("Scored 1 step (missed); opened 1 finding.")])


def test_blue_agent_scores_detection_and_opens_a_finding_end_to_end() -> None:
    with respx.mock:
        list_steps = respx.get(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/steps$").mock(
            return_value=httpx.Response(200, json=StepList(items=[Step.from_dict(_step())]).to_dict())
        )
        list_exec = respx.get(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/executions(\?.*)?$").mock(
            return_value=httpx.Response(200, json=ExecutionList(items=[
                Execution.from_dict(_execution(1, ExecutionStatus.COMPLETE, command_run="delivered weaponised .docx"))
            ]).to_dict())
        )
        patch_det = respx.patch(
            url__regex=rf"{re.escape(BASE)}/engagements/{EID}/executions/{EXECID}/detection$"
        ).mock(return_value=httpx.Response(200, json=_execution(2, ExecutionStatus.COMPLETE, outcome=ExecutionOutcome.NOT_DETECTED)))
        findings = respx.post(url__regex=rf"{re.escape(BASE)}/engagements/{EID}/findings$").mock(
            return_value=httpx.Response(201, json=_finding())
        )

        deps = blue_team_agent.BlueTeamDeps(
            client=connect(ORIGIN, token="bl_test_secret"), engagement_id=UUID(EID),
        )
        with blue_team_agent.agent.override(model=FunctionModel(_blue_script)):
            result = blue_team_agent.agent.run_sync("Score detection and file findings.", deps=deps)

    assert list_steps.called and list_exec.called and patch_det.called and findings.called

    # Blue scored the miss against the execution's current version.
    det_body = json.loads(patch_det.calls.last.request.content)
    assert det_body["version"] == 1
    assert det_body["detectionCategory"] == "none"
    assert det_body["protection"] == "not_blocked"

    # The finding is linked to the execution that revealed the gap.
    find_body = json.loads(findings.calls.last.request.content)
    assert find_body["createdFromExecution"] == EXECID
    assert find_body["severity"] == "high"

    assert "finding" in result.output.lower()


# --------------------------------------------------------------------------- #
# The error contract both agents rely on: a documented API failure comes back
# parsed as a Problem, and the wrapper turns it into a ModelRetry so the agent
# can course-correct instead of crashing.
# --------------------------------------------------------------------------- #
def test_problem_response_becomes_a_model_retry() -> None:
    problem = Problem(type_="about:blank", title="Conflict", status=409, code=ProblemCode.CONFLICT, detail="closed")
    with pytest.raises(ModelRetry, match="closed"):
        red_team_agent._unwrap(problem, "Creating the scenario")
    with pytest.raises(ModelRetry, match="undocumented"):
        blue_team_agent._unwrap(None, "Scoring detection")
