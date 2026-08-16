from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_source import ContentSource
from ...models.problem import Problem
from ...models.update_content_source_request import UpdateContentSourceRequest
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    source_id: UUID,
    *,
    body: UpdateContentSourceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/content/sources/{source_id}".format(
            source_id=quote(str(source_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSource | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSource.from_dict(response.json())

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
    body: UpdateContentSourceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSource | Problem]:
    """Edit a content source's name, URL or ref.

     Administrators only (`content.manage`). A patch: a field that is absent
    is left alone. `kind` is not a field of this request — kinds are a
    closed set seeded by migration, and there is no way to ask to change
    one.

    Builtin upstream seeds may have their mirror URL or ref retargeted so a
    deployment can point at an internal cache. The custom source accepts a
    rename the same way.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateContentSourceRequest): A patch. Every field is optional; an absent field is
            left alone. `kind`
            is deliberately not here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSource | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        body=body,
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
    body: UpdateContentSourceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSource | Problem | None:
    """Edit a content source's name, URL or ref.

     Administrators only (`content.manage`). A patch: a field that is absent
    is left alone. `kind` is not a field of this request — kinds are a
    closed set seeded by migration, and there is no way to ask to change
    one.

    Builtin upstream seeds may have their mirror URL or ref retargeted so a
    deployment can point at an internal cache. The custom source accepts a
    rename the same way.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateContentSourceRequest): A patch. Every field is optional; an absent field is
            left alone. `kind`
            is deliberately not here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSource | Problem
    """

    return sync_detailed(
        source_id=source_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateContentSourceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSource | Problem]:
    """Edit a content source's name, URL or ref.

     Administrators only (`content.manage`). A patch: a field that is absent
    is left alone. `kind` is not a field of this request — kinds are a
    closed set seeded by migration, and there is no way to ask to change
    one.

    Builtin upstream seeds may have their mirror URL or ref retargeted so a
    deployment can point at an internal cache. The custom source accepts a
    rename the same way.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateContentSourceRequest): A patch. Every field is optional; an absent field is
            left alone. `kind`
            is deliberately not here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSource | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateContentSourceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSource | Problem | None:
    """Edit a content source's name, URL or ref.

     Administrators only (`content.manage`). A patch: a field that is absent
    is left alone. `kind` is not a field of this request — kinds are a
    closed set seeded by migration, and there is no way to ask to change
    one.

    Builtin upstream seeds may have their mirror URL or ref retargeted so a
    deployment can point at an internal cache. The custom source accepts a
    rename the same way.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateContentSourceRequest): A patch. Every field is optional; an absent field is
            left alone. `kind`
            is deliberately not here.

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
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
