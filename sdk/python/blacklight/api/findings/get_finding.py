from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.finding import Finding
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    finding_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/findings/{finding_id}".format(
            finding_id=quote(str(finding_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Finding | Problem | None:
    if response.status_code == 200:
        response_200 = Finding.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Finding | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Finding | Problem]:
    """Read a single finding.

     Any engagement member may read. Returns the finding with its linked
    step ids.

    Args:
        finding_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Finding | Problem]
    """

    kwargs = _get_kwargs(
        finding_id=finding_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Finding | Problem | None:
    """Read a single finding.

     Any engagement member may read. Returns the finding with its linked
    step ids.

    Args:
        finding_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Finding | Problem
    """

    return sync_detailed(
        finding_id=finding_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Finding | Problem]:
    """Read a single finding.

     Any engagement member may read. Returns the finding with its linked
    step ids.

    Args:
        finding_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Finding | Problem]
    """

    kwargs = _get_kwargs(
        finding_id=finding_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Finding | Problem | None:
    """Read a single finding.

     Any engagement member may read. Returns the finding with its linked
    step ids.

    Args:
        finding_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Finding | Problem
    """

    return (
        await asyncio_detailed(
            finding_id=finding_id,
            client=client,
        )
    ).parsed
