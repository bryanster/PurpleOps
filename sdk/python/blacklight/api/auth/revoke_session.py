from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    session_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/auth/sessions/{session_id}".format(
            session_id=quote(str(session_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one of your own sessions.

     Takes effect at that browser's next request: the row is revoked, and
    every request resolves its session from the row rather than from
    anything cached.

    A session belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does, and for the same reason
    `DELETE /auth/tokens/{tokenId}` does: an endpoint that answered `403`
    for real identifiers and `404` for invented ones would be a way to find
    out which are real.

    Revoking the session making the request is allowed and is a way to sign
    out — but it leaves this browser holding a cookie nothing accepts and
    nothing told it to drop, so `POST /auth/logout` is the operation for
    that. Clients should offer it for the other rows.

    Args:
        session_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        session_id=session_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one of your own sessions.

     Takes effect at that browser's next request: the row is revoked, and
    every request resolves its session from the row rather than from
    anything cached.

    A session belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does, and for the same reason
    `DELETE /auth/tokens/{tokenId}` does: an endpoint that answered `403`
    for real identifiers and `404` for invented ones would be a way to find
    out which are real.

    Revoking the session making the request is allowed and is a way to sign
    out — but it leaves this browser holding a cookie nothing accepts and
    nothing told it to drop, so `POST /auth/logout` is the operation for
    that. Clients should offer it for the other rows.

    Args:
        session_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        session_id=session_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one of your own sessions.

     Takes effect at that browser's next request: the row is revoked, and
    every request resolves its session from the row rather than from
    anything cached.

    A session belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does, and for the same reason
    `DELETE /auth/tokens/{tokenId}` does: an endpoint that answered `403`
    for real identifiers and `404` for invented ones would be a way to find
    out which are real.

    Revoking the session making the request is allowed and is a way to sign
    out — but it leaves this browser holding a cookie nothing accepts and
    nothing told it to drop, so `POST /auth/logout` is the operation for
    that. Clients should offer it for the other rows.

    Args:
        session_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        session_id=session_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one of your own sessions.

     Takes effect at that browser's next request: the row is revoked, and
    every request resolves its session from the row rather than from
    anything cached.

    A session belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does, and for the same reason
    `DELETE /auth/tokens/{tokenId}` does: an endpoint that answered `403`
    for real identifiers and `404` for invented ones would be a way to find
    out which are real.

    Revoking the session making the request is allowed and is a way to sign
    out — but it leaves this browser holding a cookie nothing accepts and
    nothing told it to drop, so `POST /auth/logout` is the operation for
    that. Clients should offer it for the other rows.

    Args:
        session_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            session_id=session_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
