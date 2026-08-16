from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.import_plan_request import ImportPlanRequest
from ...models.import_plan_result import ImportPlanResult
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    body: ImportPlanRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/engagements/{engagement_id}/import-plan".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ImportPlanResult | Problem | None:
    if response.status_code == 201:
        response_201 = ImportPlanResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ImportPlanResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportPlanRequest,
) -> Response[ImportPlanResult | Problem]:
    """Import a CTID emulation plan into a new Scenario.

     Lead, red and platform administrators. Loads a CTID emulation plan from
    the content catalog, asserts the engagement's ATT&CK pin, and snapshots
    every step — creating a Scenario plus N Steps with pending Executions in
    one transaction.

    The plan is found by `planId` (content catalog surrogate id) and/or by
    `planExternalId` + `sourceId` (reference by external key). At least one
    must be provided.

    The resulting Scenario has `source=ctid` and `sourceRef` set from the
    plan lineage. Each Step snapshots name, description, procedure, and
    `techniqueExternalId` from the catalog step. `attackVersion` uses the
    engagement pin at import time — **not** the plan's advisory metadata.

    Steps whose `techniqueExternalId` does not resolve in the engagement's
    pinned ATT&CK version are still imported (technique/tactic/subtechnique
    fields left empty), with a per-step `warn` entry in the response.

    A disabled or missing plan returns 404; a plan whose source is disabled
    returns 409. Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ImportPlanRequest): Request to import a CTID emulation plan as an engagement
            Scenario.
            At least one of `planId` or (`planExternalId` + `sourceId`) must be
            provided. If both are given `planId` takes precedence.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportPlanResult | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportPlanRequest,
) -> ImportPlanResult | Problem | None:
    """Import a CTID emulation plan into a new Scenario.

     Lead, red and platform administrators. Loads a CTID emulation plan from
    the content catalog, asserts the engagement's ATT&CK pin, and snapshots
    every step — creating a Scenario plus N Steps with pending Executions in
    one transaction.

    The plan is found by `planId` (content catalog surrogate id) and/or by
    `planExternalId` + `sourceId` (reference by external key). At least one
    must be provided.

    The resulting Scenario has `source=ctid` and `sourceRef` set from the
    plan lineage. Each Step snapshots name, description, procedure, and
    `techniqueExternalId` from the catalog step. `attackVersion` uses the
    engagement pin at import time — **not** the plan's advisory metadata.

    Steps whose `techniqueExternalId` does not resolve in the engagement's
    pinned ATT&CK version are still imported (technique/tactic/subtechnique
    fields left empty), with a per-step `warn` entry in the response.

    A disabled or missing plan returns 404; a plan whose source is disabled
    returns 409. Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ImportPlanRequest): Request to import a CTID emulation plan as an engagement
            Scenario.
            At least one of `planId` or (`planExternalId` + `sourceId`) must be
            provided. If both are given `planId` takes precedence.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportPlanResult | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportPlanRequest,
) -> Response[ImportPlanResult | Problem]:
    """Import a CTID emulation plan into a new Scenario.

     Lead, red and platform administrators. Loads a CTID emulation plan from
    the content catalog, asserts the engagement's ATT&CK pin, and snapshots
    every step — creating a Scenario plus N Steps with pending Executions in
    one transaction.

    The plan is found by `planId` (content catalog surrogate id) and/or by
    `planExternalId` + `sourceId` (reference by external key). At least one
    must be provided.

    The resulting Scenario has `source=ctid` and `sourceRef` set from the
    plan lineage. Each Step snapshots name, description, procedure, and
    `techniqueExternalId` from the catalog step. `attackVersion` uses the
    engagement pin at import time — **not** the plan's advisory metadata.

    Steps whose `techniqueExternalId` does not resolve in the engagement's
    pinned ATT&CK version are still imported (technique/tactic/subtechnique
    fields left empty), with a per-step `warn` entry in the response.

    A disabled or missing plan returns 404; a plan whose source is disabled
    returns 409. Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ImportPlanRequest): Request to import a CTID emulation plan as an engagement
            Scenario.
            At least one of `planId` or (`planExternalId` + `sourceId`) must be
            provided. If both are given `planId` takes precedence.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportPlanResult | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportPlanRequest,
) -> ImportPlanResult | Problem | None:
    """Import a CTID emulation plan into a new Scenario.

     Lead, red and platform administrators. Loads a CTID emulation plan from
    the content catalog, asserts the engagement's ATT&CK pin, and snapshots
    every step — creating a Scenario plus N Steps with pending Executions in
    one transaction.

    The plan is found by `planId` (content catalog surrogate id) and/or by
    `planExternalId` + `sourceId` (reference by external key). At least one
    must be provided.

    The resulting Scenario has `source=ctid` and `sourceRef` set from the
    plan lineage. Each Step snapshots name, description, procedure, and
    `techniqueExternalId` from the catalog step. `attackVersion` uses the
    engagement pin at import time — **not** the plan's advisory metadata.

    Steps whose `techniqueExternalId` does not resolve in the engagement's
    pinned ATT&CK version are still imported (technique/tactic/subtechnique
    fields left empty), with a per-step `warn` entry in the response.

    A disabled or missing plan returns 404; a plan whose source is disabled
    returns 409. Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ImportPlanRequest): Request to import a CTID emulation plan as an engagement
            Scenario.
            At least one of `planId` or (`planExternalId` + `sourceId`) must be
            provided. If both are given `planId` takes precedence.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportPlanResult | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            body=body,
        )
    ).parsed
