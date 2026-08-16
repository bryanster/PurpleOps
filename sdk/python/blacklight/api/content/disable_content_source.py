from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_source import ContentSource
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    source_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/content/sources/{source_id}/disable".format(
            source_id=quote(str(source_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSource | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSource.from_dict(response.json())

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
) -> Response[ContentSource | Problem]:
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
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSource | Problem]:
    """Disable a content source.

     Administrators only (`content.manage`). Sets `enabled=false`. Idempotent.

    Rows stay on disk. Library browse/search/pickers (later tickets) omit
    objects from disabled sources by default, and APIs that would create a
    *new* reference answer `409`. Existing engagement data is not modified.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSource | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSource | Problem | None:
    """Disable a content source.

     Administrators only (`content.manage`). Sets `enabled=false`. Idempotent.

    Rows stay on disk. Library browse/search/pickers (later tickets) omit
    objects from disabled sources by default, and APIs that would create a
    *new* reference answer `409`. Existing engagement data is not modified.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSource | Problem
    """

    return sync_detailed(
        source_id=source_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSource | Problem]:
    """Disable a content source.

     Administrators only (`content.manage`). Sets `enabled=false`. Idempotent.

    Rows stay on disk. Library browse/search/pickers (later tickets) omit
    objects from disabled sources by default, and APIs that would create a
    *new* reference answer `409`. Existing engagement data is not modified.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSource | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSource | Problem | None:
    """Disable a content source.

     Administrators only (`content.manage`). Sets `enabled=false`. Idempotent.

    Rows stay on disk. Library browse/search/pickers (later tickets) omit
    objects from disabled sources by default, and APIs that would create a
    *new* reference answer `409`. Existing engagement data is not modified.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSource | Problem
    """

    return (
        await asyncio_detailed(
            source_id=source_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
