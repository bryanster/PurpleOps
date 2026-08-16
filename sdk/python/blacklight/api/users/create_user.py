from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.create_user_request import CreateUserRequest
from ...models.created_user import CreatedUser
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: CreateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/users",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> CreatedUser | Problem | None:
    if response.status_code == 201:
        response_201 = CreatedUser.from_dict(response.json())

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
) -> Response[CreatedUser | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[CreatedUser | Problem]:
    """Create an account.

     Administrators only. There is no email transport in this deployment, so
    nothing is sent: the response carries `inviteUrl`, which is where this
    installation is signed in to, and the administrator passes it on
    themselves along with whatever credential they chose.

    Two shapes of account, and `password` is what picks between them:

    - **With a password.** A local account, `active` by default. Tell the
      person the address, the password and the link, and they change the
      password once they are in.
    - **Without one.** An account that signs in through the identity
      provider, `invited` by default — it exists and nobody has claimed it.
      The first successful single sign-on that resolves to this address
      claims it, and the account becomes `active` at that moment.

    An address already in use — in any casing, with any surrounding
    whitespace — is `409`, never a 500 and never a second account.

    Args:
        x_csrf_token (str | Unset):
        body (CreateUserRequest): Body of `POST /users`. The identifier, the timestamps and the
            invite
            link are the server's; everything a caller chooses is here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreatedUser | Problem]
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
    body: CreateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> CreatedUser | Problem | None:
    """Create an account.

     Administrators only. There is no email transport in this deployment, so
    nothing is sent: the response carries `inviteUrl`, which is where this
    installation is signed in to, and the administrator passes it on
    themselves along with whatever credential they chose.

    Two shapes of account, and `password` is what picks between them:

    - **With a password.** A local account, `active` by default. Tell the
      person the address, the password and the link, and they change the
      password once they are in.
    - **Without one.** An account that signs in through the identity
      provider, `invited` by default — it exists and nobody has claimed it.
      The first successful single sign-on that resolves to this address
      claims it, and the account becomes `active` at that moment.

    An address already in use — in any casing, with any surrounding
    whitespace — is `409`, never a 500 and never a second account.

    Args:
        x_csrf_token (str | Unset):
        body (CreateUserRequest): Body of `POST /users`. The identifier, the timestamps and the
            invite
            link are the server's; everything a caller chooses is here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreatedUser | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[CreatedUser | Problem]:
    """Create an account.

     Administrators only. There is no email transport in this deployment, so
    nothing is sent: the response carries `inviteUrl`, which is where this
    installation is signed in to, and the administrator passes it on
    themselves along with whatever credential they chose.

    Two shapes of account, and `password` is what picks between them:

    - **With a password.** A local account, `active` by default. Tell the
      person the address, the password and the link, and they change the
      password once they are in.
    - **Without one.** An account that signs in through the identity
      provider, `invited` by default — it exists and nobody has claimed it.
      The first successful single sign-on that resolves to this address
      claims it, and the account becomes `active` at that moment.

    An address already in use — in any casing, with any surrounding
    whitespace — is `409`, never a 500 and never a second account.

    Args:
        x_csrf_token (str | Unset):
        body (CreateUserRequest): Body of `POST /users`. The identifier, the timestamps and the
            invite
            link are the server's; everything a caller chooses is here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreatedUser | Problem]
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
    body: CreateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> CreatedUser | Problem | None:
    """Create an account.

     Administrators only. There is no email transport in this deployment, so
    nothing is sent: the response carries `inviteUrl`, which is where this
    installation is signed in to, and the administrator passes it on
    themselves along with whatever credential they chose.

    Two shapes of account, and `password` is what picks between them:

    - **With a password.** A local account, `active` by default. Tell the
      person the address, the password and the link, and they change the
      password once they are in.
    - **Without one.** An account that signs in through the identity
      provider, `invited` by default — it exists and nobody has claimed it.
      The first successful single sign-on that resolves to this address
      claims it, and the account becomes `active` at that moment.

    An address already in use — in any casing, with any surrounding
    whitespace — is `409`, never a 500 and never a second account.

    Args:
        x_csrf_token (str | Unset):
        body (CreateUserRequest): Body of `POST /users`. The identifier, the timestamps and the
            invite
            link are the server's; everything a caller chooses is here.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreatedUser | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
