from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_emulation_plan_detail import ContentEmulationPlanDetail
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    plan_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/emulation-plans/{plan_id}".format(
            plan_id=quote(str(plan_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentEmulationPlanDetail | Problem | None:
    if response.status_code == 200:
        response_200 = ContentEmulationPlanDetail.from_dict(response.json())

        return response_200

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
) -> Response[ContentEmulationPlanDetail | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    plan_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentEmulationPlanDetail | Problem]:
    """Read one emulation plan with ordered steps.

     Any authenticated subject. Plans from disabled sources answer `404`.

    The response includes steps sorted by `ordinal` ascending (1-based
    document order from the upstream plan YAML). Each step may carry a
    structured `procedure` payload (platforms, executors/commands, input
    args) for M3 scenario import — never executed by Blacklight.

    Args:
        plan_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentEmulationPlanDetail | Problem]
    """

    kwargs = _get_kwargs(
        plan_id=plan_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    plan_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentEmulationPlanDetail | Problem | None:
    """Read one emulation plan with ordered steps.

     Any authenticated subject. Plans from disabled sources answer `404`.

    The response includes steps sorted by `ordinal` ascending (1-based
    document order from the upstream plan YAML). Each step may carry a
    structured `procedure` payload (platforms, executors/commands, input
    args) for M3 scenario import — never executed by Blacklight.

    Args:
        plan_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentEmulationPlanDetail | Problem
    """

    return sync_detailed(
        plan_id=plan_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    plan_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentEmulationPlanDetail | Problem]:
    """Read one emulation plan with ordered steps.

     Any authenticated subject. Plans from disabled sources answer `404`.

    The response includes steps sorted by `ordinal` ascending (1-based
    document order from the upstream plan YAML). Each step may carry a
    structured `procedure` payload (platforms, executors/commands, input
    args) for M3 scenario import — never executed by Blacklight.

    Args:
        plan_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentEmulationPlanDetail | Problem]
    """

    kwargs = _get_kwargs(
        plan_id=plan_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    plan_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentEmulationPlanDetail | Problem | None:
    """Read one emulation plan with ordered steps.

     Any authenticated subject. Plans from disabled sources answer `404`.

    The response includes steps sorted by `ordinal` ascending (1-based
    document order from the upstream plan YAML). Each step may carry a
    structured `procedure` payload (platforms, executors/commands, input
    args) for M3 scenario import — never executed by Blacklight.

    Args:
        plan_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentEmulationPlanDetail | Problem
    """

    return (
        await asyncio_detailed(
            plan_id=plan_id,
            client=client,
        )
    ).parsed
