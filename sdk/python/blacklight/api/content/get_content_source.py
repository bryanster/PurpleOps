from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_source_detail import ContentSourceDetail
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    source_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/sources/{source_id}".format(
            source_id=quote(str(source_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSourceDetail | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSourceDetail.from_dict(response.json())

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
) -> Response[ContentSourceDetail | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentSourceDetail | Problem]:
    """Read one content source.

     Any authenticated subject. Carries license/attribution and a summary of
    the most recent sync job, when one exists.

    Args:
        source_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSourceDetail | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentSourceDetail | Problem | None:
    """Read one content source.

     Any authenticated subject. Carries license/attribution and a summary of
    the most recent sync job, when one exists.

    Args:
        source_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSourceDetail | Problem
    """

    return sync_detailed(
        source_id=source_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentSourceDetail | Problem]:
    """Read one content source.

     Any authenticated subject. Carries license/attribution and a summary of
    the most recent sync job, when one exists.

    Args:
        source_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSourceDetail | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentSourceDetail | Problem | None:
    """Read one content source.

     Any authenticated subject. Carries license/attribution and a summary of
    the most recent sync job, when one exists.

    Args:
        source_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSourceDetail | Problem
    """

    return (
        await asyncio_detailed(
            source_id=source_id,
            client=client,
        )
    ).parsed
