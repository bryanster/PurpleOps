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
from uuid import UUID


def _get_kwargs(
    user_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/users/{user_id}/sessions/revoke".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RevokedSessions | None:
    if response.status_code == 200:
        response_200 = RevokedSessions.from_dict(response.json())

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
) -> Response[Problem | RevokedSessions]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RevokedSessions]:
    """Sign an account out everywhere.

     Administrators only. Ends every live session the account holds, at once
    and without disabling it — what an administrator reaches for when a
    laptop goes missing and the person still needs their account tomorrow.

    Service tokens are not sessions and are not touched. Use
    `POST /users/{userId}/disable` to stop those as well.

    Idempotent: an account with nothing live answers `200` and `revoked: 0`.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RevokedSessions]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RevokedSessions | None:
    """Sign an account out everywhere.

     Administrators only. Ends every live session the account holds, at once
    and without disabling it — what an administrator reaches for when a
    laptop goes missing and the person still needs their account tomorrow.

    Service tokens are not sessions and are not touched. Use
    `POST /users/{userId}/disable` to stop those as well.

    Idempotent: an account with nothing live answers `200` and `revoked: 0`.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RevokedSessions
    """

    return sync_detailed(
        user_id=user_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RevokedSessions]:
    """Sign an account out everywhere.

     Administrators only. Ends every live session the account holds, at once
    and without disabling it — what an administrator reaches for when a
    laptop goes missing and the person still needs their account tomorrow.

    Service tokens are not sessions and are not touched. Use
    `POST /users/{userId}/disable` to stop those as well.

    Idempotent: an account with nothing live answers `200` and `revoked: 0`.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RevokedSessions]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RevokedSessions | None:
    """Sign an account out everywhere.

     Administrators only. Ends every live session the account holds, at once
    and without disabling it — what an administrator reaches for when a
    laptop goes missing and the person still needs their account tomorrow.

    Service tokens are not sessions and are not touched. Use
    `POST /users/{userId}/disable` to stop those as well.

    Idempotent: an account with nothing live answers `200` and `revoked: 0`.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RevokedSessions
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
