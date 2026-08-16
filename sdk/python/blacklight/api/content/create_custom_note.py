from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_note import ContentNote
from ...models.create_custom_note_request import CreateCustomNoteRequest
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: CreateCustomNoteRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/content/custom/notes",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> ContentNote | Problem | None:
    if response.status_code == 201:
        response_201 = ContentNote.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

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
) -> Response[ContentNote | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomNoteRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentNote | Problem]:
    """Create a custom knowledge-base note.

     Administrators only (`content.manage`). Markdown body size is capped
    by `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` (default 256KiB). Technique
    external id is optional; when present must look like a MITRE id.

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomNoteRequest): Body for creating a custom knowledge-base note.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentNote | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomNoteRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentNote | Problem | None:
    """Create a custom knowledge-base note.

     Administrators only (`content.manage`). Markdown body size is capped
    by `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` (default 256KiB). Technique
    external id is optional; when present must look like a MITRE id.

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomNoteRequest): Body for creating a custom knowledge-base note.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentNote | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomNoteRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentNote | Problem]:
    """Create a custom knowledge-base note.

     Administrators only (`content.manage`). Markdown body size is capped
    by `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` (default 256KiB). Technique
    external id is optional; when present must look like a MITRE id.

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomNoteRequest): Body for creating a custom knowledge-base note.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentNote | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomNoteRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentNote | Problem | None:
    """Create a custom knowledge-base note.

     Administrators only (`content.manage`). Markdown body size is capped
    by `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` (default 256KiB). Technique
    external id is optional; when present must look like a MITRE id.

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomNoteRequest): Body for creating a custom knowledge-base note.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentNote | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
