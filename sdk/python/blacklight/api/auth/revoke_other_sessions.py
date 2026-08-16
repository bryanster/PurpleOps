from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.revoked_sessions import RevokedSessions
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/sessions/revoke-others",
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RevokedSessions | None:
    if response.status_code == 200:
        response_200 = RevokedSessions.from_dict(response.json())

        return response_200

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
) -> Response[Problem | RevokedSessions]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RevokedSessions]:
    """Sign yourself out everywhere except here.

     Ends every session you hold except the one making the request, and
    reports how many. The browser doing the asking keeps working, which is
    what separates this from signing out.

    It is the one control that is useful without reading the list first: the
    person who has just remembered a laptop in a hotel does not need to
    identify which row it is.

    Idempotent, and `revoked: 0` is a normal answer for somebody signed in
    only here. Service tokens are not sessions and are untouched — those are
    `DELETE /auth/tokens/{tokenId}`.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RevokedSessions]
    """

    kwargs = _get_kwargs(
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RevokedSessions | None:
    """Sign yourself out everywhere except here.

     Ends every session you hold except the one making the request, and
    reports how many. The browser doing the asking keeps working, which is
    what separates this from signing out.

    It is the one control that is useful without reading the list first: the
    person who has just remembered a laptop in a hotel does not need to
    identify which row it is.

    Idempotent, and `revoked: 0` is a normal answer for somebody signed in
    only here. Service tokens are not sessions and are untouched — those are
    `DELETE /auth/tokens/{tokenId}`.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RevokedSessions
    """

    return sync_detailed(
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RevokedSessions]:
    """Sign yourself out everywhere except here.

     Ends every session you hold except the one making the request, and
    reports how many. The browser doing the asking keeps working, which is
    what separates this from signing out.

    It is the one control that is useful without reading the list first: the
    person who has just remembered a laptop in a hotel does not need to
    identify which row it is.

    Idempotent, and `revoked: 0` is a normal answer for somebody signed in
    only here. Service tokens are not sessions and are untouched — those are
    `DELETE /auth/tokens/{tokenId}`.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RevokedSessions]
    """

    kwargs = _get_kwargs(
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RevokedSessions | None:
    """Sign yourself out everywhere except here.

     Ends every session you hold except the one making the request, and
    reports how many. The browser doing the asking keeps working, which is
    what separates this from signing out.

    It is the one control that is useful without reading the list first: the
    person who has just remembered a laptop in a hotel does not need to
    identify which row it is.

    Idempotent, and `revoked: 0` is a normal answer for somebody signed in
    only here. Service tokens are not sessions and are untouched — those are
    `DELETE /auth/tokens/{tokenId}`.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RevokedSessions
    """

    return (
        await asyncio_detailed(
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
