from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_tactic import ContentTactic
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    tactic_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/tactics/{tactic_id}".format(
            tactic_id=quote(str(tactic_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentTactic | Problem | None:
    if response.status_code == 200:
        response_200 = ContentTactic.from_dict(response.json())

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
) -> Response[ContentTactic | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    tactic_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentTactic | Problem]:
    """Read one ATT&CK tactic.

    Args:
        tactic_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTactic | Problem]
    """

    kwargs = _get_kwargs(
        tactic_id=tactic_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    tactic_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentTactic | Problem | None:
    """Read one ATT&CK tactic.

    Args:
        tactic_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTactic | Problem
    """

    return sync_detailed(
        tactic_id=tactic_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    tactic_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentTactic | Problem]:
    """Read one ATT&CK tactic.

    Args:
        tactic_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTactic | Problem]
    """

    kwargs = _get_kwargs(
        tactic_id=tactic_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    tactic_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentTactic | Problem | None:
    """Read one ATT&CK tactic.

    Args:
        tactic_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTactic | Problem
    """

    return (
        await asyncio_detailed(
            tactic_id=tactic_id,
            client=client,
        )
    ).parsed
