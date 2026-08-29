# Examples

Two [Pydantic AI](https://ai.pydantic.dev) agents that drive the generated Python
SDK to work the two sides of the same Blacklight engagement. They form a loop: the
red agent records attack steps, then the blue agent scores detection on them and
opens findings.

| Agent | Fills in | Needs |
|---|---|---|
| `red_team_agent.py` | The red side — scenarios, ATT&CK steps, executions | a token with lead/red role; creates a demo engagement if none is pinned |
| `blue_team_agent.py` | The blue side — detection scoring and findings | a token with lead/blue role; **requires** an engagement red has already recorded into |

Neither agent performs an attack or reads a real SIEM: everything is *recorded in
Blacklight*. Run `red_team_agent.py` first, note the engagement id it prints, then
point `blue_team_agent.py` at that engagement.

## `red_team_agent.py` — a Pydantic AI red-team agent

An example agent, built on [Pydantic AI](https://ai.pydantic.dev), that turns a
natural-language adversary-emulation objective into **recorded attack steps in
Blacklight**. It plans an ATT&CK-mapped chain and drives the generated Python SDK
to write the red side of the workbook — opening a scenario, registering each step,
and recording what red executed — so blue can score detection against it.

It does **not** run any offensive action. "Executing a step" here means recording
in Blacklight that a step was run (the command, the hosts, the outcome). The
offensive actions are assumed to happen out of band on an authorised range.

### The tools the agent has

The agent is given three tools, each a thin wrapper over one SDK operation. They
thread ids back to the model, so credentials and the target engagement stay in the
process (injected via `RunContext`) and never enter the model's context:

| Tool | SDK operation | What it does |
|---|---|---|
| `open_scenario` | `POST /engagements/{id}/scenarios` | Opens a scenario for the operation, returns its id |
| `register_attack_step` | `POST …/scenarios/{id}/steps` | Registers an ATT&CK-mapped step; returns the `execution_id` + `version` of the sibling execution row Blacklight auto-creates |
| `record_execution` | `PATCH …/executions/{id}/execution` | Records the red execution — command, hosts, status — using optimistic-concurrency `version` |

Documented API failures come back parsed as a `Problem` (not raised); the wrappers
surface those as a `ModelRetry` so the agent can self-correct (e.g. fix a bad
technique id) instead of the run crashing.

### Running it

From `sdk/python` so `import blacklight` resolves:

```sh
pip install -r examples/requirements.txt          # pydantic-ai; the SDK's own deps too if needed
export ANTHROPIC_API_KEY=sk-ant-...               # Pydantic AI reads this; model defaults to claude-opus-5
export BLACKLIGHT_URL=https://blacklight.example.com
export BLACKLIGHT_TOKEN=bl_<prefix>_<secret>       # a scoped service token, lead/red role

# optional — write into an existing engagement instead of creating a demo one:
export BLACKLIGHT_ENGAGEMENT_ID=<uuid>
# optional — pick a different model:
export RED_TEAM_MODEL=anthropic:claude-sonnet-5

python examples/red_team_agent.py \
  "Emulate a phishing-to-exfil chain against the finance VLAN using a Windows foothold."
```

The token's principal needs the lead or red role on the engagement — the same
authorisation a human red operator needs to write the workbook. With no argument
the agent uses a built-in default objective.

### Notes

- `Agent.run(...).output` is the Pydantic AI ≥ 0.2 accessor; on older versions it
  is `.data`.
- The service token is sent as `Authorization: Bearer` on every request. Scope and
  expire it in Blacklight; never use a session cookie for automation.
- List endpoints can send `items: null` for an empty page — the example reads them
  as `.items or []`.

## `blue_team_agent.py` — a Pydantic AI blue-team agent

The counterpart. It reads the executions red logged and fills in the blue side of
the same workbook: it scores detection on each execution and opens findings for the
gaps. As with the red agent, credentials and the target engagement are injected via
`RunContext` — only execution ids reach the model.

| Tool | SDK operation | What it does |
|---|---|---|
| `review_recorded_steps` | `list_engagement_steps` + `list_engagement_executions` | Joins steps to executions; returns each with its ATT&CK technique, red's command, and the `execution_id` + `version` to score against |
| `score_detection` | `PATCH …/executions/{id}/detection` | Records detection category, protection, MTTD (`detected_at`), the alerting source/rule, and severity |
| `open_finding` | `POST /engagements/{id}/findings` | Opens a finding for a gap, linked to the execution that revealed it (`created_from_execution`) |

Detection outcome (`detected`/`prevented`/`not_detected`) and mean-time-to-detect
are derived server-side from what `score_detection` writes — the blue agent supplies
the category, protection, and `detected_at`, and Blacklight computes the rest.

### Running it

From `sdk/python`:

```sh
export ANTHROPIC_API_KEY=sk-ant-...
export BLACKLIGHT_URL=https://blacklight.example.com
export BLACKLIGHT_TOKEN=bl_<prefix>_<secret>       # a scoped service token, lead/blue role
export BLACKLIGHT_ENGAGEMENT_ID=<uuid>             # required: the engagement to score
# optional — pick a different model:
export BLUE_TEAM_MODEL=anthropic:claude-sonnet-5

python examples/blue_team_agent.py
```

Unlike the red agent, the engagement id is **required** — blue scores existing work
rather than creating it. This example reasons over each step's ATT&CK context to
produce a realistic assessment; in a real deployment you would replace that
reasoning with lookups against your EDR/SIEM (or feed the agent those results).

## Testing the agents

`test_agents.py` proves the agents *can actually work* without a live model or a
live Blacklight. Exactly two things are faked:

- **The model** — a Pydantic AI `FunctionModel` scripts the precise tool calls, so
  no `ANTHROPIC_API_KEY` and no LLM are involved (a placeholder key in `conftest.py`
  only satisfies the eager `Agent(...)` constructor at import; `agent.override(...)`
  swaps the model out entirely).
- **Blacklight** — `respx` intercepts httpx and answers each endpoint with a real
  SDK model serialized to its wire JSON.

Everything else runs for real: the agent, its tools, the generated SDK operations,
and the whole model → tool → SDK → HTTP request → response-parse → tool-return loop.
The tests assert the right requests were built (URLs, ATT&CK id, optimistic-
concurrency `version`, the finding's `createdFromExecution` link) and that each
agent completes its workflow. A third test pins the error contract both agents rely
on — a documented API failure comes back as a `Problem` and the wrapper turns it
into a `ModelRetry`.

Run from `sdk/python`:

```sh
# with pip-installed deps (examples/requirements.txt):
pytest examples/

# or with the SDK's own uv dev group plus pydantic-ai:
uv run --project sdk/python --group dev --with pydantic-ai pytest examples/
```

The `pytest examples/` path is explicit, so it runs regardless of the SDK's
`testpaths = ["tests"]` setting.

