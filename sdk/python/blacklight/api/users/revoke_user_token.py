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
    user_id: UUID,
    token_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/users/{user_id}/tokens/{token_id}".format(
            user_id=quote(str(user_id), safe=""),
            token_id=quote(str(token_id), safe=""),
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
    user_id: UUID,
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one service token belonging to somebody else.

     Administrators only. Takes effect on the token's next request, the same
    as the owner revoking it themselves: nothing caches a token anywhere, so
    there is no window in which it still works. The owner's own listing then
    shows it as revoked, carrying the same timestamp — there is one
    revocation and one row, not an administrative copy of one.

    `revokedBy` on the token records who ended it, which is what makes an
    administrative revocation tellable from a routine rotation afterwards.
    The activity log says the same thing under `token.admin_revoked`.

    Idempotent, and it keeps the original revocation time and revoker: the
    first revocation is when access actually stopped, and whoever arrived
    second stopped nothing.

    `tokenId` must belong to `userId`. A token belonging to a different
    account answers `404`, exactly as an identifier that names nothing does
    — so this is not a way to revoke by identifier alone, and not a way to
    find out which identifiers are real.

    As with the listing, a service token cannot call this. That matters most
    here: a leaked credential belonging to an administrator would otherwise
    be able to end every other credential in the installation.

    Args:
        user_id (UUID):
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        token_id=token_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    user_id: UUID,
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one service token belonging to somebody else.

     Administrators only. Takes effect on the token's next request, the same
    as the owner revoking it themselves: nothing caches a token anywhere, so
    there is no window in which it still works. The owner's own listing then
    shows it as revoked, carrying the same timestamp — there is one
    revocation and one row, not an administrative copy of one.

    `revokedBy` on the token records who ended it, which is what makes an
    administrative revocation tellable from a routine rotation afterwards.
    The activity log says the same thing under `token.admin_revoked`.

    Idempotent, and it keeps the original revocation time and revoker: the
    first revocation is when access actually stopped, and whoever arrived
    second stopped nothing.

    `tokenId` must belong to `userId`. A token belonging to a different
    account answers `404`, exactly as an identifier that names nothing does
    — so this is not a way to revoke by identifier alone, and not a way to
    find out which identifiers are real.

    As with the listing, a service token cannot call this. That matters most
    here: a leaked credential belonging to an administrator would otherwise
    be able to end every other credential in the installation.

    Args:
        user_id (UUID):
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        user_id=user_id,
        token_id=token_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one service token belonging to somebody else.

     Administrators only. Takes effect on the token's next request, the same
    as the owner revoking it themselves: nothing caches a token anywhere, so
    there is no window in which it still works. The owner's own listing then
    shows it as revoked, carrying the same timestamp — there is one
    revocation and one row, not an administrative copy of one.

    `revokedBy` on the token records who ended it, which is what makes an
    administrative revocation tellable from a routine rotation afterwards.
    The activity log says the same thing under `token.admin_revoked`.

    Idempotent, and it keeps the original revocation time and revoker: the
    first revocation is when access actually stopped, and whoever arrived
    second stopped nothing.

    `tokenId` must belong to `userId`. A token belonging to a different
    account answers `404`, exactly as an identifier that names nothing does
    — so this is not a way to revoke by identifier alone, and not a way to
    find out which identifiers are real.

    As with the listing, a service token cannot call this. That matters most
    here: a leaked credential belonging to an administrator would otherwise
    be able to end every other credential in the installation.

    Args:
        user_id (UUID):
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        token_id=token_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one service token belonging to somebody else.

     Administrators only. Takes effect on the token's next request, the same
    as the owner revoking it themselves: nothing caches a token anywhere, so
    there is no window in which it still works. The owner's own listing then
    shows it as revoked, carrying the same timestamp — there is one
    revocation and one row, not an administrative copy of one.

    `revokedBy` on the token records who ended it, which is what makes an
    administrative revocation tellable from a routine rotation afterwards.
    The activity log says the same thing under `token.admin_revoked`.

    Idempotent, and it keeps the original revocation time and revoker: the
    first revocation is when access actually stopped, and whoever arrived
    second stopped nothing.

    `tokenId` must belong to `userId`. A token belonging to a different
    account answers `404`, exactly as an identifier that names nothing does
    — so this is not a way to revoke by identifier alone, and not a way to
    find out which identifiers are real.

    As with the listing, a service token cannot call this. That matters most
    here: a leaked credential belonging to an administrator would otherwise
    be able to end every other credential in the installation.

    Args:
        user_id (UUID):
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            token_id=token_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
