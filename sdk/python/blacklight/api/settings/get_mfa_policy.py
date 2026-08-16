from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.mfa_policy import MFAPolicy
from ...models.problem import Problem
from typing import cast


def _get_kwargs() -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/settings/mfa",
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> MFAPolicy | Problem | None:
    if response.status_code == 200:
        response_200 = MFAPolicy.from_dict(response.json())

        return response_200

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[MFAPolicy | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[MFAPolicy | Problem]:
    """Read the platform-wide multi-factor authentication policy.

     Administrators only. What every ordinary caller needs to know about the
    requirement — whether it applies to *them* — is on `GET /auth/me` as
    `mfa.required`, which is a fact about one person rather than the policy
    that produced it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MFAPolicy | Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> MFAPolicy | Problem | None:
    """Read the platform-wide multi-factor authentication policy.

     Administrators only. What every ordinary caller needs to know about the
    requirement — whether it applies to *them* — is on `GET /auth/me` as
    `mfa.required`, which is a fact about one person rather than the policy
    that produced it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MFAPolicy | Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[MFAPolicy | Problem]:
    """Read the platform-wide multi-factor authentication policy.

     Administrators only. What every ordinary caller needs to know about the
    requirement — whether it applies to *them* — is on `GET /auth/me` as
    `mfa.required`, which is a fact about one person rather than the policy
    that produced it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MFAPolicy | Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> MFAPolicy | Problem | None:
    """Read the platform-wide multi-factor authentication policy.

     Administrators only. What every ordinary caller needs to know about the
    requirement — whether it applies to *them* — is on `GET /auth/me` as
    `mfa.required`, which is a fact about one person rather than the policy
    that produced it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MFAPolicy | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
