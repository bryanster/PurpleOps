from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.current_user import CurrentUser
from ...models.problem import Problem
from typing import cast


def _get_kwargs() -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/me",
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> CurrentUser | Problem | None:
    if response.status_code == 200:
        response_200 = CurrentUser.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[CurrentUser | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[CurrentUser | Problem]:
    """Return the signed-in user, their platform role and their engagement memberships.

     Everything the interface needs to decide what to show: who you are, what
    you may do to this installation, which engagements you are in and with
    which role, and whether this session has satisfied MFA.

    It never returns anything about the session token. What is in the cookie
    stays in the cookie.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CurrentUser | Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> CurrentUser | Problem | None:
    """Return the signed-in user, their platform role and their engagement memberships.

     Everything the interface needs to decide what to show: who you are, what
    you may do to this installation, which engagements you are in and with
    which role, and whether this session has satisfied MFA.

    It never returns anything about the session token. What is in the cookie
    stays in the cookie.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CurrentUser | Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[CurrentUser | Problem]:
    """Return the signed-in user, their platform role and their engagement memberships.

     Everything the interface needs to decide what to show: who you are, what
    you may do to this installation, which engagements you are in and with
    which role, and whether this session has satisfied MFA.

    It never returns anything about the session token. What is in the cookie
    stays in the cookie.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CurrentUser | Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> CurrentUser | Problem | None:
    """Return the signed-in user, their platform role and their engagement memberships.

     Everything the interface needs to decide what to show: who you are, what
    you may do to this installation, which engagements you are in and with
    which role, and whether this session has satisfied MFA.

    It never returns anything about the session token. What is in the cookie
    stays in the cookie.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CurrentUser | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
