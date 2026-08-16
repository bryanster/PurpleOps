from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.platform_role import PlatformRole
from ...models.problem import Problem
from ...models.user_page import UserPage
from ...models.user_status import UserStatus
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    status: UserStatus | Unset = UNSET,
    role: PlatformRole | Unset = UNSET,
    q: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["limit"] = limit

    params["cursor"] = cursor

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    json_role: str | Unset = UNSET
    if not isinstance(role, Unset):
        json_role = role.value

    params["role"] = json_role

    params["q"] = q

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/users",
        "params": params,
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | UserPage | None:
    if response.status_code == 200:
        response_200 = UserPage.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | UserPage]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    status: UserStatus | Unset = UNSET,
    role: PlatformRole | Unset = UNSET,
    q: str | Unset = UNSET,
) -> Response[Problem | UserPage]:
    """List the accounts on this installation.

     Administrators only. Every account, oldest first, with the standard
    cursor pagination — `limit` is capped at 200 by the shared parameter, so
    a caller cannot ask for the whole table in one request.

    Narrow it with `status`, `role` and `q`. The search matches the display
    name or the email address, without regard to case, anywhere in either.

    No response here carries a password hash, an authenticator secret, a
    recovery code or a session token. None of those is a field of the `User`
    schema, which is what makes that a property of the document rather than
    a promise about the implementation.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        status (UserStatus | Unset): Whether an account can be used. Retirement is a status change
            and never
            a deletion: the executions, comments and findings somebody wrote keep
            their author (`M1-001`).
        role (PlatformRole | Unset): What somebody may do to this installation: `admin` manages
            users, content
            and every engagement; `member` takes part in the engagements they belong
            to. What they may do *inside* one is `EngagementRole`, and the two are
            deliberately not the same vocabulary.
        q (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UserPage]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        status=status,
        role=role,
        q=q,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    status: UserStatus | Unset = UNSET,
    role: PlatformRole | Unset = UNSET,
    q: str | Unset = UNSET,
) -> Problem | UserPage | None:
    """List the accounts on this installation.

     Administrators only. Every account, oldest first, with the standard
    cursor pagination — `limit` is capped at 200 by the shared parameter, so
    a caller cannot ask for the whole table in one request.

    Narrow it with `status`, `role` and `q`. The search matches the display
    name or the email address, without regard to case, anywhere in either.

    No response here carries a password hash, an authenticator secret, a
    recovery code or a session token. None of those is a field of the `User`
    schema, which is what makes that a property of the document rather than
    a promise about the implementation.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        status (UserStatus | Unset): Whether an account can be used. Retirement is a status change
            and never
            a deletion: the executions, comments and findings somebody wrote keep
            their author (`M1-001`).
        role (PlatformRole | Unset): What somebody may do to this installation: `admin` manages
            users, content
            and every engagement; `member` takes part in the engagements they belong
            to. What they may do *inside* one is `EngagementRole`, and the two are
            deliberately not the same vocabulary.
        q (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UserPage
    """

    return sync_detailed(
        client=client,
        limit=limit,
        cursor=cursor,
        status=status,
        role=role,
        q=q,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    status: UserStatus | Unset = UNSET,
    role: PlatformRole | Unset = UNSET,
    q: str | Unset = UNSET,
) -> Response[Problem | UserPage]:
    """List the accounts on this installation.

     Administrators only. Every account, oldest first, with the standard
    cursor pagination — `limit` is capped at 200 by the shared parameter, so
    a caller cannot ask for the whole table in one request.

    Narrow it with `status`, `role` and `q`. The search matches the display
    name or the email address, without regard to case, anywhere in either.

    No response here carries a password hash, an authenticator secret, a
    recovery code or a session token. None of those is a field of the `User`
    schema, which is what makes that a property of the document rather than
    a promise about the implementation.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        status (UserStatus | Unset): Whether an account can be used. Retirement is a status change
            and never
            a deletion: the executions, comments and findings somebody wrote keep
            their author (`M1-001`).
        role (PlatformRole | Unset): What somebody may do to this installation: `admin` manages
            users, content
            and every engagement; `member` takes part in the engagements they belong
            to. What they may do *inside* one is `EngagementRole`, and the two are
            deliberately not the same vocabulary.
        q (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UserPage]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        status=status,
        role=role,
        q=q,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    status: UserStatus | Unset = UNSET,
    role: PlatformRole | Unset = UNSET,
    q: str | Unset = UNSET,
) -> Problem | UserPage | None:
    """List the accounts on this installation.

     Administrators only. Every account, oldest first, with the standard
    cursor pagination — `limit` is capped at 200 by the shared parameter, so
    a caller cannot ask for the whole table in one request.

    Narrow it with `status`, `role` and `q`. The search matches the display
    name or the email address, without regard to case, anywhere in either.

    No response here carries a password hash, an authenticator secret, a
    recovery code or a session token. None of those is a field of the `User`
    schema, which is what makes that a property of the document rather than
    a promise about the implementation.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        status (UserStatus | Unset): Whether an account can be used. Retirement is a status change
            and never
            a deletion: the executions, comments and findings somebody wrote keep
            their author (`M1-001`).
        role (PlatformRole | Unset): What somebody may do to this installation: `admin` manages
            users, content
            and every engagement; `member` takes part in the engagements they belong
            to. What they may do *inside* one is `EngagementRole`, and the two are
            deliberately not the same vocabulary.
        q (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UserPage
    """

    return (
        await asyncio_detailed(
            client=client,
            limit=limit,
            cursor=cursor,
            status=status,
            role=role,
            q=q,
        )
    ).parsed
