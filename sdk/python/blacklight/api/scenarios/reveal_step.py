from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.step import Step
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    scenario_id: UUID,
    step_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/engagements/{engagement_id}/scenarios/{scenario_id}/steps/{step_id}/reveal".format(
            engagement_id=quote(str(engagement_id), safe=""),
            scenario_id=quote(str(scenario_id), safe=""),
            step_id=quote(str(step_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | Step | None:
    if response.status_code == 200:
        response_200 = Step.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | Step]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    scenario_id: UUID,
    step_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | Step]:
    """Reveal a step to the blue team.

     Lead, red and platform administrators. Sets `revealed_at` to the
    current time. Idempotent: calling on an already-revealed step
    succeeds with no change. Activity: step.revealed.

    Args:
        engagement_id (UUID):
        scenario_id (UUID):
        step_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Step]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        scenario_id=scenario_id,
        step_id=step_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    scenario_id: UUID,
    step_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | Step | None:
    """Reveal a step to the blue team.

     Lead, red and platform administrators. Sets `revealed_at` to the
    current time. Idempotent: calling on an already-revealed step
    succeeds with no change. Activity: step.revealed.

    Args:
        engagement_id (UUID):
        scenario_id (UUID):
        step_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Step
    """

    return sync_detailed(
        engagement_id=engagement_id,
        scenario_id=scenario_id,
        step_id=step_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    scenario_id: UUID,
    step_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | Step]:
    """Reveal a step to the blue team.

     Lead, red and platform administrators. Sets `revealed_at` to the
    current time. Idempotent: calling on an already-revealed step
    succeeds with no change. Activity: step.revealed.

    Args:
        engagement_id (UUID):
        scenario_id (UUID):
        step_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Step]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        scenario_id=scenario_id,
        step_id=step_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    scenario_id: UUID,
    step_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | Step | None:
    """Reveal a step to the blue team.

     Lead, red and platform administrators. Sets `revealed_at` to the
    current time. Idempotent: calling on an already-revealed step
    succeeds with no change. Activity: step.revealed.

    Args:
        engagement_id (UUID):
        scenario_id (UUID):
        step_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Step
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            scenario_id=scenario_id,
            step_id=step_id,
            client=client,
        )
    ).parsed
