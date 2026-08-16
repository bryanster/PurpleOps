from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.update_user_request import UpdateUserRequest
from ...models.user import User
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    user_id: UUID,
    *,
    body: UpdateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/users/{user_id}".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

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
    body: UpdateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | User]:
    """Edit an account's name, platform role, status or MFA requirement.

     Administrators only, and a patch rather than a replacement: a field that
    is absent is left alone, so two administrators editing different things
    at once do not overwrite each other's change.

    The email address is not editable here. It is the identifier a federated
    sign-in links an account by, and changing it would move somebody else's
    single sign-on onto this account.

    Changing `platformRole` takes effect on the target's **existing**
    session, at their next request, without them signing in again — the role
    is read from the account on every request and nothing caches it. The
    same is true in the other direction: a promotion applies immediately too.

    Setting `status` to `disabled` does what `POST /users/{userId}/disable`
    does, including revoking the account's sessions.

    The last account that is both `admin` and `active` cannot be demoted or
    disabled: `409`, saying so. An installation with no administrator is one
    nobody can administer, and no request should be able to produce it.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateUserRequest): Body of `PATCH /users/{userId}`. Every field is optional and an
            absent
            one is left alone, so two administrators editing different things do not
            overwrite each other. At least one must be present — an empty patch is a
            client bug, and answering it `200` would hide one.

            `email` is deliberately not a field: it is what a federated sign-in
            links an account by, so editing it could move somebody else's single
            sign-on onto this account.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        body=body,
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
    body: UpdateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | User | None:
    """Edit an account's name, platform role, status or MFA requirement.

     Administrators only, and a patch rather than a replacement: a field that
    is absent is left alone, so two administrators editing different things
    at once do not overwrite each other's change.

    The email address is not editable here. It is the identifier a federated
    sign-in links an account by, and changing it would move somebody else's
    single sign-on onto this account.

    Changing `platformRole` takes effect on the target's **existing**
    session, at their next request, without them signing in again — the role
    is read from the account on every request and nothing caches it. The
    same is true in the other direction: a promotion applies immediately too.

    Setting `status` to `disabled` does what `POST /users/{userId}/disable`
    does, including revoking the account's sessions.

    The last account that is both `admin` and `active` cannot be demoted or
    disabled: `409`, saying so. An installation with no administrator is one
    nobody can administer, and no request should be able to produce it.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateUserRequest): Body of `PATCH /users/{userId}`. Every field is optional and an
            absent
            one is left alone, so two administrators editing different things do not
            overwrite each other. At least one must be present — an empty patch is a
            client bug, and answering it `200` would hide one.

            `email` is deliberately not a field: it is what a federated sign-in
            links an account by, so editing it could move somebody else's single
            sign-on onto this account.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | User
    """

    return sync_detailed(
        user_id=user_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | User]:
    """Edit an account's name, platform role, status or MFA requirement.

     Administrators only, and a patch rather than a replacement: a field that
    is absent is left alone, so two administrators editing different things
    at once do not overwrite each other's change.

    The email address is not editable here. It is the identifier a federated
    sign-in links an account by, and changing it would move somebody else's
    single sign-on onto this account.

    Changing `platformRole` takes effect on the target's **existing**
    session, at their next request, without them signing in again — the role
    is read from the account on every request and nothing caches it. The
    same is true in the other direction: a promotion applies immediately too.

    Setting `status` to `disabled` does what `POST /users/{userId}/disable`
    does, including revoking the account's sessions.

    The last account that is both `admin` and `active` cannot be demoted or
    disabled: `409`, saying so. An installation with no administrator is one
    nobody can administer, and no request should be able to produce it.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateUserRequest): Body of `PATCH /users/{userId}`. Every field is optional and an
            absent
            one is left alone, so two administrators editing different things do not
            overwrite each other. At least one must be present — an empty patch is a
            client bug, and answering it `200` would hide one.

            `email` is deliberately not a field: it is what a federated sign-in
            links an account by, so editing it could move somebody else's single
            sign-on onto this account.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateUserRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | User | None:
    """Edit an account's name, platform role, status or MFA requirement.

     Administrators only, and a patch rather than a replacement: a field that
    is absent is left alone, so two administrators editing different things
    at once do not overwrite each other's change.

    The email address is not editable here. It is the identifier a federated
    sign-in links an account by, and changing it would move somebody else's
    single sign-on onto this account.

    Changing `platformRole` takes effect on the target's **existing**
    session, at their next request, without them signing in again — the role
    is read from the account on every request and nothing caches it. The
    same is true in the other direction: a promotion applies immediately too.

    Setting `status` to `disabled` does what `POST /users/{userId}/disable`
    does, including revoking the account's sessions.

    The last account that is both `admin` and `active` cannot be demoted or
    disabled: `409`, saying so. An installation with no administrator is one
    nobody can administer, and no request should be able to produce it.

    Args:
        user_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateUserRequest): Body of `PATCH /users/{userId}`. Every field is optional and an
            absent
            one is left alone, so two administrators editing different things do not
            overwrite each other. At least one must be present — an empty patch is a
            client bug, and answering it `200` would hide one.

            `email` is deliberately not a field: it is what a federated sign-in
            links an account by, so editing it could move somebody else's single
            sign-on onto this account.

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
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
