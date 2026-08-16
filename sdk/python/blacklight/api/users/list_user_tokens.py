from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.service_tokens import ServiceTokens
from typing import cast
from uuid import UUID


def _get_kwargs(
    user_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/users/{user_id}/tokens".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ServiceTokens | None:
    if response.status_code == 200:
        response_200 = ServiceTokens.from_dict(response.json())

        return response_200

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
) -> Response[Problem | ServiceTokens]:
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
) -> Response[Problem | ServiceTokens]:
    """List the service tokens one account holds.

     Administrators only, and the question an incident starts with: what does
    this account hold, and when was each of them last used? Until this
    endpoint the answer needed a SQL console, at exactly the moment nobody
    wants to be reaching for one.

    The same shape and the same renderer as `GET /auth/tokens`, including
    the expired and revoked rows — an administrator working from a
    different set from the owner's is an administrator working from a
    second account of what exists. No secret is here, for the reason no
    secret is there: the server keeps a hash and could not produce one if
    asked.

    What this is *instead of* is disabling the account. Disabling stops
    every token it holds, and it stops the person too; this stops one
    credential and leaves them able to work.

    A service token cannot call this, whatever it is scoped for and however
    senior its owner — `403`. Reading which credentials exist is the
    business of a person at a keyboard, for the reason
    `POST /auth/tokens` says.

    Args:
        user_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ServiceTokens]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ServiceTokens | None:
    """List the service tokens one account holds.

     Administrators only, and the question an incident starts with: what does
    this account hold, and when was each of them last used? Until this
    endpoint the answer needed a SQL console, at exactly the moment nobody
    wants to be reaching for one.

    The same shape and the same renderer as `GET /auth/tokens`, including
    the expired and revoked rows — an administrator working from a
    different set from the owner's is an administrator working from a
    second account of what exists. No secret is here, for the reason no
    secret is there: the server keeps a hash and could not produce one if
    asked.

    What this is *instead of* is disabling the account. Disabling stops
    every token it holds, and it stops the person too; this stops one
    credential and leaves them able to work.

    A service token cannot call this, whatever it is scoped for and however
    senior its owner — `403`. Reading which credentials exist is the
    business of a person at a keyboard, for the reason
    `POST /auth/tokens` says.

    Args:
        user_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ServiceTokens
    """

    return sync_detailed(
        user_id=user_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | ServiceTokens]:
    """List the service tokens one account holds.

     Administrators only, and the question an incident starts with: what does
    this account hold, and when was each of them last used? Until this
    endpoint the answer needed a SQL console, at exactly the moment nobody
    wants to be reaching for one.

    The same shape and the same renderer as `GET /auth/tokens`, including
    the expired and revoked rows — an administrator working from a
    different set from the owner's is an administrator working from a
    second account of what exists. No secret is here, for the reason no
    secret is there: the server keeps a hash and could not produce one if
    asked.

    What this is *instead of* is disabling the account. Disabling stops
    every token it holds, and it stops the person too; this stops one
    credential and leaves them able to work.

    A service token cannot call this, whatever it is scoped for and however
    senior its owner — `403`. Reading which credentials exist is the
    business of a person at a keyboard, for the reason
    `POST /auth/tokens` says.

    Args:
        user_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ServiceTokens]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ServiceTokens | None:
    """List the service tokens one account holds.

     Administrators only, and the question an incident starts with: what does
    this account hold, and when was each of them last used? Until this
    endpoint the answer needed a SQL console, at exactly the moment nobody
    wants to be reaching for one.

    The same shape and the same renderer as `GET /auth/tokens`, including
    the expired and revoked rows — an administrator working from a
    different set from the owner's is an administrator working from a
    second account of what exists. No secret is here, for the reason no
    secret is there: the server keeps a hash and could not produce one if
    asked.

    What this is *instead of* is disabling the account. Disabling stops
    every token it holds, and it stops the person too; this stops one
    credential and leaves them able to work.

    A service token cannot call this, whatever it is scoped for and however
    senior its owner — `403`. Reading which credentials exist is the
    business of a person at a keyboard, for the reason
    `POST /auth/tokens` says.

    Args:
        user_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ServiceTokens
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            client=client,
        )
    ).parsed
