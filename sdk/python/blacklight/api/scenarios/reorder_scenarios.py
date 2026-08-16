from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.reorder_scenarios import ReorderScenarios
from ...models.scenario_list import ScenarioList
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    body: ReorderScenarios,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/engagements/{engagement_id}/scenarios/order".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | ScenarioList | None:
    if response.status_code == 200:
        response_200 = ScenarioList.from_dict(response.json())

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
) -> Response[Problem | ScenarioList]:
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
    body: ReorderScenarios,
) -> Response[Problem | ScenarioList]:
    """Reorder scenarios in an engagement.

     Lead, red and platform administrators. The body lists every scenario
    id in the desired order. Ordinals are reassigned 1..N to match.
    Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ReorderScenarios):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ScenarioList]
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
    body: ReorderScenarios,
) -> Problem | ScenarioList | None:
    """Reorder scenarios in an engagement.

     Lead, red and platform administrators. The body lists every scenario
    id in the desired order. Ordinals are reassigned 1..N to match.
    Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ReorderScenarios):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ScenarioList
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
    body: ReorderScenarios,
) -> Response[Problem | ScenarioList]:
    """Reorder scenarios in an engagement.

     Lead, red and platform administrators. The body lists every scenario
    id in the desired order. Ordinals are reassigned 1..N to match.
    Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ReorderScenarios):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ScenarioList]
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
    body: ReorderScenarios,
) -> Problem | ScenarioList | None:
    """Reorder scenarios in an engagement.

     Lead, red and platform administrators. The body lists every scenario
    id in the desired order. Ordinals are reassigned 1..N to match.
    Closed/archived engagements return 409.

    Args:
        engagement_id (UUID):
        body (ReorderScenarios):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ScenarioList
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            body=body,
        )
    ).parsed
