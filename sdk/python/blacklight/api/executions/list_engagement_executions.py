from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.execution_list import ExecutionList
from ...models.execution_status import ExecutionStatus
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    scenario_id: UUID | Unset = UNSET,
    status: ExecutionStatus | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_scenario_id: str | Unset = UNSET
    if not isinstance(scenario_id, Unset):
        json_scenario_id = str(scenario_id)
    params["scenarioId"] = json_scenario_id

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/executions".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ExecutionList | Problem | None:
    if response.status_code == 200:
        response_200 = ExecutionList.from_dict(response.json())

        return response_200

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ExecutionList | Problem]:
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
    scenario_id: UUID | Unset = UNSET,
    status: ExecutionStatus | Unset = UNSET,
) -> Response[ExecutionList | Problem]:
    """List executions in an engagement.

     Members and platform administrators. Filters by scenario and/or
    status. In a blind engagement, blue members only see executions
    for revealed steps.

    Args:
        engagement_id (UUID):
        scenario_id (UUID | Unset):
        status (ExecutionStatus | Unset): The red-side state of one execution.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ExecutionList | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        scenario_id=scenario_id,
        status=status,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    scenario_id: UUID | Unset = UNSET,
    status: ExecutionStatus | Unset = UNSET,
) -> ExecutionList | Problem | None:
    """List executions in an engagement.

     Members and platform administrators. Filters by scenario and/or
    status. In a blind engagement, blue members only see executions
    for revealed steps.

    Args:
        engagement_id (UUID):
        scenario_id (UUID | Unset):
        status (ExecutionStatus | Unset): The red-side state of one execution.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ExecutionList | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        scenario_id=scenario_id,
        status=status,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    scenario_id: UUID | Unset = UNSET,
    status: ExecutionStatus | Unset = UNSET,
) -> Response[ExecutionList | Problem]:
    """List executions in an engagement.

     Members and platform administrators. Filters by scenario and/or
    status. In a blind engagement, blue members only see executions
    for revealed steps.

    Args:
        engagement_id (UUID):
        scenario_id (UUID | Unset):
        status (ExecutionStatus | Unset): The red-side state of one execution.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ExecutionList | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        scenario_id=scenario_id,
        status=status,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    scenario_id: UUID | Unset = UNSET,
    status: ExecutionStatus | Unset = UNSET,
) -> ExecutionList | Problem | None:
    """List executions in an engagement.

     Members and platform administrators. Filters by scenario and/or
    status. In a blind engagement, blue members only see executions
    for revealed steps.

    Args:
        engagement_id (UUID):
        scenario_id (UUID | Unset):
        status (ExecutionStatus | Unset): The red-side state of one execution.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ExecutionList | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            scenario_id=scenario_id,
            status=status,
        )
    ).parsed
