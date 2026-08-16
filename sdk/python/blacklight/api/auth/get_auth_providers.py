from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.auth_providers import AuthProviders
from ...models.problem import Problem
from typing import cast


def _get_kwargs() -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/providers",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AuthProviders | Problem | None:
    if response.status_code == 200:
        response_200 = AuthProviders.from_dict(response.json())

        return response_200

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AuthProviders | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[AuthProviders | Problem]:
    """List the ways this deployment can be signed in to.

     Public, because it is what the login page reads *before* anybody has
    signed in — it is the list of buttons to draw.

    `password` is whether local sign-in is offered, and `sso` is one entry
    per configured single sign-on provider, each with the URL to send the
    browser to.

    A provider that is configured but cannot be reached is **absent** from
    `sso` (M1-009). That is deliberate and it is the point of this endpoint:
    a button that leads to a provider which is down is worse than no button,
    and an identity provider having a bad morning must never be the reason
    nobody can get in. It reappears on its own, with no restart, once the
    provider answers again.

    It reveals that a deployment has single sign-on and which protocol, which
    is visible from the sign-in page of every such product and is not a
    secret. It reveals nothing about the client, the issuer or the mapping.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AuthProviders | Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> AuthProviders | Problem | None:
    """List the ways this deployment can be signed in to.

     Public, because it is what the login page reads *before* anybody has
    signed in — it is the list of buttons to draw.

    `password` is whether local sign-in is offered, and `sso` is one entry
    per configured single sign-on provider, each with the URL to send the
    browser to.

    A provider that is configured but cannot be reached is **absent** from
    `sso` (M1-009). That is deliberate and it is the point of this endpoint:
    a button that leads to a provider which is down is worse than no button,
    and an identity provider having a bad morning must never be the reason
    nobody can get in. It reappears on its own, with no restart, once the
    provider answers again.

    It reveals that a deployment has single sign-on and which protocol, which
    is visible from the sign-in page of every such product and is not a
    secret. It reveals nothing about the client, the issuer or the mapping.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AuthProviders | Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[AuthProviders | Problem]:
    """List the ways this deployment can be signed in to.

     Public, because it is what the login page reads *before* anybody has
    signed in — it is the list of buttons to draw.

    `password` is whether local sign-in is offered, and `sso` is one entry
    per configured single sign-on provider, each with the URL to send the
    browser to.

    A provider that is configured but cannot be reached is **absent** from
    `sso` (M1-009). That is deliberate and it is the point of this endpoint:
    a button that leads to a provider which is down is worse than no button,
    and an identity provider having a bad morning must never be the reason
    nobody can get in. It reappears on its own, with no restart, once the
    provider answers again.

    It reveals that a deployment has single sign-on and which protocol, which
    is visible from the sign-in page of every such product and is not a
    secret. It reveals nothing about the client, the issuer or the mapping.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AuthProviders | Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> AuthProviders | Problem | None:
    """List the ways this deployment can be signed in to.

     Public, because it is what the login page reads *before* anybody has
    signed in — it is the list of buttons to draw.

    `password` is whether local sign-in is offered, and `sso` is one entry
    per configured single sign-on provider, each with the URL to send the
    browser to.

    A provider that is configured but cannot be reached is **absent** from
    `sso` (M1-009). That is deliberate and it is the point of this endpoint:
    a button that leads to a provider which is down is worse than no button,
    and an identity provider having a bad morning must never be the reason
    nobody can get in. It reappears on its own, with no restart, once the
    provider answers again.

    It reveals that a deployment has single sign-on and which protocol, which
    is visible from the sign-in page of every such product and is not a
    secret. It reveals nothing about the client, the issuer or the mapping.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AuthProviders | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
