from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from typing import cast


def _get_kwargs() -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/saml/metadata",
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | None:
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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem]:
    """Serve this deployment's SAML service provider metadata.

     The XML an identity provider administrator uploads, or points their
    console at, to register this deployment as a service provider (M1-010).
    It carries the entity ID, the assertion consumer service URL and the
    signing certificate.

    It is public and it is meant to be: every field in it is something the
    identity provider is about to be told anyway, and half of them are
    already visible in the authentication request the browser carries. The
    **private** key never appears here — only the certificate, which is the
    public half.

    `404` when this deployment has no SAML configured.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | None:
    """Serve this deployment's SAML service provider metadata.

     The XML an identity provider administrator uploads, or points their
    console at, to register this deployment as a service provider (M1-010).
    It carries the entity ID, the assertion consumer service URL and the
    signing certificate.

    It is public and it is meant to be: every field in it is something the
    identity provider is about to be told anyway, and half of them are
    already visible in the authentication request the browser carries. The
    **private** key never appears here — only the certificate, which is the
    public half.

    `404` when this deployment has no SAML configured.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem]:
    """Serve this deployment's SAML service provider metadata.

     The XML an identity provider administrator uploads, or points their
    console at, to register this deployment as a service provider (M1-010).
    It carries the entity ID, the assertion consumer service URL and the
    signing certificate.

    It is public and it is meant to be: every field in it is something the
    identity provider is about to be told anyway, and half of them are
    already visible in the authentication request the browser carries. The
    **private** key never appears here — only the certificate, which is the
    public half.

    `404` when this deployment has no SAML configured.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | None:
    """Serve this deployment's SAML service provider metadata.

     The XML an identity provider administrator uploads, or points their
    console at, to register this deployment as a service provider (M1-010).
    It carries the entity ID, the assertion consumer service URL and the
    signing certificate.

    It is public and it is meant to be: every field in it is something the
    identity provider is about to be told anyway, and half of them are
    already visible in the authentication request the browser carries. The
    **private** key never appears here — only the certificate, which is the
    public half.

    `404` when this deployment has no SAML configured.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
