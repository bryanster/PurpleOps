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
    token_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/auth/tokens/{token_id}".format(
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
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one of your own service tokens.

     Takes effect on the token's next request: there is no cached copy of it
    anywhere, so there is no window during which it still works.

    Idempotent — revoking a token that has already been revoked answers the
    same way, and keeps the original revocation time, because the first
    revocation is when access actually stopped.

    A token belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does. The two are indistinguishable on
    purpose: an endpoint that answered `403` for real identifiers and `404`
    for invented ones would be a way to find out which are real. An
    administrator revoking somebody else's uses
    `DELETE /users/{userId}/tokens/{tokenId}`, which names the account.

    As with creation, a service token cannot call this.

    Args:
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        token_id=token_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one of your own service tokens.

     Takes effect on the token's next request: there is no cached copy of it
    anywhere, so there is no window during which it still works.

    Idempotent — revoking a token that has already been revoked answers the
    same way, and keeps the original revocation time, because the first
    revocation is when access actually stopped.

    A token belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does. The two are indistinguishable on
    purpose: an endpoint that answered `403` for real identifiers and `404`
    for invented ones would be a way to find out which are real. An
    administrator revoking somebody else's uses
    `DELETE /users/{userId}/tokens/{tokenId}`, which names the account.

    As with creation, a service token cannot call this.

    Args:
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        token_id=token_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke one of your own service tokens.

     Takes effect on the token's next request: there is no cached copy of it
    anywhere, so there is no window during which it still works.

    Idempotent — revoking a token that has already been revoked answers the
    same way, and keeps the original revocation time, because the first
    revocation is when access actually stopped.

    A token belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does. The two are indistinguishable on
    purpose: an endpoint that answered `403` for real identifiers and `404`
    for invented ones would be a way to find out which are real. An
    administrator revoking somebody else's uses
    `DELETE /users/{userId}/tokens/{tokenId}`, which names the account.

    As with creation, a service token cannot call this.

    Args:
        token_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        token_id=token_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    token_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke one of your own service tokens.

     Takes effect on the token's next request: there is no cached copy of it
    anywhere, so there is no window during which it still works.

    Idempotent — revoking a token that has already been revoked answers the
    same way, and keeps the original revocation time, because the first
    revocation is when access actually stopped.

    A token belonging to somebody else answers `404`, exactly as an
    identifier that names nothing does. The two are indistinguishable on
    purpose: an endpoint that answered `403` for real identifiers and `404`
    for invented ones would be a way to find out which are real. An
    administrator revoking somebody else's uses
    `DELETE /users/{userId}/tokens/{tokenId}`, which names the account.

    As with creation, a service token cannot call this.

    Args:
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
            token_id=token_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
