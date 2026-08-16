from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.evidence import Evidence
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    execution_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/executions/{execution_id}/evidence".format(
            execution_id=quote(str(execution_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | list[Evidence] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = Evidence.from_dict(response_200_item_data)

            response_200.append(response_200_item)

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
) -> Response[Problem | list[Evidence]]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[Evidence]]:
    """List evidence for an execution.

     Any engagement member who can read the execution. Evidence on
    unrevealed steps in blind mode is concealed (same rule as
    execution.read). Newest first.

    Args:
        execution_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Evidence]]
    """

    kwargs = _get_kwargs(
        execution_id=execution_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[Evidence] | None:
    """List evidence for an execution.

     Any engagement member who can read the execution. Evidence on
    unrevealed steps in blind mode is concealed (same rule as
    execution.read). Newest first.

    Args:
        execution_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Evidence]
    """

    return sync_detailed(
        execution_id=execution_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[Evidence]]:
    """List evidence for an execution.

     Any engagement member who can read the execution. Evidence on
    unrevealed steps in blind mode is concealed (same rule as
    execution.read). Newest first.

    Args:
        execution_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Evidence]]
    """

    kwargs = _get_kwargs(
        execution_id=execution_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[Evidence] | None:
    """List evidence for an execution.

     Any engagement member who can read the execution. Evidence on
    unrevealed steps in blind mode is concealed (same rule as
    execution.read). Newest first.

    Args:
        execution_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Evidence]
    """

    return (
        await asyncio_detailed(
            execution_id=execution_id,
            client=client,
        )
    ).parsed
