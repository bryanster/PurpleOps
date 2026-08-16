from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.user import User
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
        "url": "/users/{user_id}/enable".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | User | None:
    if response.status_code == 200:
        response_200 = User.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | User]:
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
) -> Response[Problem | User]:
    """Enable an account.

     Administrators only. The account becomes `active`. Sessions revoked
    while it was disabled stay revoked — enabling somebody gives them their
    account back, not the browser tab they had open a month ago.

    Idempotent, and the way an `invited` account that will never use single
    sign-on is turned on by hand.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
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
) -> Problem | User | None:
    """Enable an account.

     Administrators only. The account becomes `active`. Sessions revoked
    while it was disabled stay revoked — enabling somebody gives them their
    account back, not the browser tab they had open a month ago.

    Idempotent, and the way an `invited` account that will never use single
    sign-on is turned on by hand.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | User
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
) -> Response[Problem | User]:
    """Enable an account.

     Administrators only. The account becomes `active`. Sessions revoked
    while it was disabled stay revoked — enabling somebody gives them their
    account back, not the browser tab they had open a month ago.

    Idempotent, and the way an `invited` account that will never use single
    sign-on is turned on by hand.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
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
) -> Problem | User | None:
    """Enable an account.

     Administrators only. The account becomes `active`. Sessions revoked
    while it was disabled stay revoked — enabling somebody gives them their
    account back, not the browser tab they had open a month ago.

    Idempotent, and the way an `invited` account that will never use single
    sign-on is turned on by hand.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | User
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
