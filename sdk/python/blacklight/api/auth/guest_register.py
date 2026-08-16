from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.guest_register_request import GuestRegisterRequest
from ...models.guest_register_result import GuestRegisterResult
from ...models.problem import Problem
from typing import cast


def _get_kwargs(
    *,
    body: GuestRegisterRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/guest-register",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GuestRegisterResult | Problem | None:
    if response.status_code == 201:
        response_201 = GuestRegisterResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[GuestRegisterResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: GuestRegisterRequest,
) -> Response[GuestRegisterResult | Problem]:
    """Create a minimal local account for share invite access.

     Public. Creates a local user account with no platform role
    beyond member and no engagement memberships. Intended for
    share invite recipients who do not yet have an account.

    Rate-limited like login to prevent account enumeration.

    Args:
        body (GuestRegisterRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GuestRegisterResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: GuestRegisterRequest,
) -> GuestRegisterResult | Problem | None:
    """Create a minimal local account for share invite access.

     Public. Creates a local user account with no platform role
    beyond member and no engagement memberships. Intended for
    share invite recipients who do not yet have an account.

    Rate-limited like login to prevent account enumeration.

    Args:
        body (GuestRegisterRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GuestRegisterResult | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: GuestRegisterRequest,
) -> Response[GuestRegisterResult | Problem]:
    """Create a minimal local account for share invite access.

     Public. Creates a local user account with no platform role
    beyond member and no engagement memberships. Intended for
    share invite recipients who do not yet have an account.

    Rate-limited like login to prevent account enumeration.

    Args:
        body (GuestRegisterRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GuestRegisterResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: GuestRegisterRequest,
) -> GuestRegisterResult | Problem | None:
    """Create a minimal local account for share invite access.

     Public. Creates a local user account with no platform role
    beyond member and no engagement memberships. Intended for
    share invite recipients who do not yet have an account.

    Rate-limited like login to prevent account enumeration.

    Args:
        body (GuestRegisterRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GuestRegisterResult | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
