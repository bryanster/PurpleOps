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


def _get_kwargs(
    *,
    return_to: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["return_to"] = return_to

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/saml/start",
        "params": params,
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 302:
        response_302 = cast(Any, None)
        return response_302

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

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
    *,
    client: AuthenticatedClient | Client,
    return_to: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Begin a single sign-on through the configured SAML provider.

     Redirects the browser to the identity provider's single sign-on service
    with a signed `AuthnRequest` (HTTP-Redirect binding), and sets a
    short-lived, sealed cookie holding that request's ID and where to land
    afterwards. It is a navigation rather than a fetch: send the browser
    here, do not call it with `fetch`.

    `return_to` is where to land afterwards and must be a path within this
    application, exactly as it is for `GET /auth/oidc/start`, and for the
    same reason: an endpoint that redirects wherever a query parameter says
    is an open redirect on the login path.

    `404` when no provider is configured, or when the configured one's
    metadata could not be read. Local sign-in is unaffected either way.

    Args:
        return_to (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        return_to=return_to,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    return_to: str | Unset = UNSET,
) -> Any | Problem | None:
    """Begin a single sign-on through the configured SAML provider.

     Redirects the browser to the identity provider's single sign-on service
    with a signed `AuthnRequest` (HTTP-Redirect binding), and sets a
    short-lived, sealed cookie holding that request's ID and where to land
    afterwards. It is a navigation rather than a fetch: send the browser
    here, do not call it with `fetch`.

    `return_to` is where to land afterwards and must be a path within this
    application, exactly as it is for `GET /auth/oidc/start`, and for the
    same reason: an endpoint that redirects wherever a query parameter says
    is an open redirect on the login path.

    `404` when no provider is configured, or when the configured one's
    metadata could not be read. Local sign-in is unaffected either way.

    Args:
        return_to (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        return_to=return_to,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    return_to: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Begin a single sign-on through the configured SAML provider.

     Redirects the browser to the identity provider's single sign-on service
    with a signed `AuthnRequest` (HTTP-Redirect binding), and sets a
    short-lived, sealed cookie holding that request's ID and where to land
    afterwards. It is a navigation rather than a fetch: send the browser
    here, do not call it with `fetch`.

    `return_to` is where to land afterwards and must be a path within this
    application, exactly as it is for `GET /auth/oidc/start`, and for the
    same reason: an endpoint that redirects wherever a query parameter says
    is an open redirect on the login path.

    `404` when no provider is configured, or when the configured one's
    metadata could not be read. Local sign-in is unaffected either way.

    Args:
        return_to (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        return_to=return_to,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    return_to: str | Unset = UNSET,
) -> Any | Problem | None:
    """Begin a single sign-on through the configured SAML provider.

     Redirects the browser to the identity provider's single sign-on service
    with a signed `AuthnRequest` (HTTP-Redirect binding), and sets a
    short-lived, sealed cookie holding that request's ID and where to land
    afterwards. It is a navigation rather than a fetch: send the browser
    here, do not call it with `fetch`.

    `return_to` is where to land afterwards and must be a path within this
    application, exactly as it is for `GET /auth/oidc/start`, and for the
    same reason: an endpoint that redirects wherever a query parameter says
    is an open redirect on the login path.

    `404` when no provider is configured, or when the configured one's
    metadata could not be read. Local sign-in is unaffected either way.

    Args:
        return_to (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            return_to=return_to,
        )
    ).parsed
