from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_source_kind import ContentSourceKind
from ...models.content_source_list import ContentSourceList
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    kind: ContentSourceKind | Unset = UNSET,
    enabled: bool | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_kind: str | Unset = UNSET
    if not isinstance(kind, Unset):
        json_kind = kind.value

    params["kind"] = json_kind

    params["enabled"] = enabled

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/sources",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSourceList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSourceList.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContentSourceList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    kind: ContentSourceKind | Unset = UNSET,
    enabled: bool | Unset = UNSET,
) -> Response[ContentSourceList | Problem]:
    """List content sources.

     Any authenticated subject. The shared library is what an engagement is
    planned from, so reading the registry is not an administrative act.

    Filter by `kind` and/or `enabled`. Disabled sources stay in the list —
    an administrator needs to see them to turn them back on — and library
    browse endpoints (later tickets) are what omit their objects by default.

    Args:
        kind (ContentSourceKind | Unset): Closed vocabulary of content libraries. New kinds are a
            migration, not
            a string somebody passed to an API. There is no create-source endpoint
            in M2 — only the seeded rows.
        enabled (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSourceList | Problem]
    """

    kwargs = _get_kwargs(
        kind=kind,
        enabled=enabled,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    kind: ContentSourceKind | Unset = UNSET,
    enabled: bool | Unset = UNSET,
) -> ContentSourceList | Problem | None:
    """List content sources.

     Any authenticated subject. The shared library is what an engagement is
    planned from, so reading the registry is not an administrative act.

    Filter by `kind` and/or `enabled`. Disabled sources stay in the list —
    an administrator needs to see them to turn them back on — and library
    browse endpoints (later tickets) are what omit their objects by default.

    Args:
        kind (ContentSourceKind | Unset): Closed vocabulary of content libraries. New kinds are a
            migration, not
            a string somebody passed to an API. There is no create-source endpoint
            in M2 — only the seeded rows.
        enabled (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSourceList | Problem
    """

    return sync_detailed(
        client=client,
        kind=kind,
        enabled=enabled,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    kind: ContentSourceKind | Unset = UNSET,
    enabled: bool | Unset = UNSET,
) -> Response[ContentSourceList | Problem]:
    """List content sources.

     Any authenticated subject. The shared library is what an engagement is
    planned from, so reading the registry is not an administrative act.

    Filter by `kind` and/or `enabled`. Disabled sources stay in the list —
    an administrator needs to see them to turn them back on — and library
    browse endpoints (later tickets) are what omit their objects by default.

    Args:
        kind (ContentSourceKind | Unset): Closed vocabulary of content libraries. New kinds are a
            migration, not
            a string somebody passed to an API. There is no create-source endpoint
            in M2 — only the seeded rows.
        enabled (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSourceList | Problem]
    """

    kwargs = _get_kwargs(
        kind=kind,
        enabled=enabled,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    kind: ContentSourceKind | Unset = UNSET,
    enabled: bool | Unset = UNSET,
) -> ContentSourceList | Problem | None:
    """List content sources.

     Any authenticated subject. The shared library is what an engagement is
    planned from, so reading the registry is not an administrative act.

    Filter by `kind` and/or `enabled`. Disabled sources stay in the list —
    an administrator needs to see them to turn them back on — and library
    browse endpoints (later tickets) are what omit their objects by default.

    Args:
        kind (ContentSourceKind | Unset): Closed vocabulary of content libraries. New kinds are a
            migration, not
            a string somebody passed to an API. There is no create-source endpoint
            in M2 — only the seeded rows.
        enabled (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSourceList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            kind=kind,
            enabled=enabled,
        )
    ).parsed
