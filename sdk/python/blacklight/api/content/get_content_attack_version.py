from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_attack_version_detail import ContentAttackVersionDetail
from ...models.problem import Problem
from typing import cast


def _get_kwargs(
    version: str,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/attack/versions/{version}".format(
            version=quote(str(version), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentAttackVersionDetail | Problem | None:
    if response.status_code == 200:
        response_200 = ContentAttackVersionDetail.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContentAttackVersionDetail | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    version: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentAttackVersionDetail | Problem]:
    """Read one installed ATT&CK version and family counts.

     Any authenticated subject. Exact match on the version path param —
    `15.1` does not resolve `v15.1`. Unknown version → `404`.

    Args:
        version (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentAttackVersionDetail | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    version: str,
    *,
    client: AuthenticatedClient | Client,
) -> ContentAttackVersionDetail | Problem | None:
    """Read one installed ATT&CK version and family counts.

     Any authenticated subject. Exact match on the version path param —
    `15.1` does not resolve `v15.1`. Unknown version → `404`.

    Args:
        version (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentAttackVersionDetail | Problem
    """

    return sync_detailed(
        version=version,
        client=client,
    ).parsed


async def asyncio_detailed(
    version: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentAttackVersionDetail | Problem]:
    """Read one installed ATT&CK version and family counts.

     Any authenticated subject. Exact match on the version path param —
    `15.1` does not resolve `v15.1`. Unknown version → `404`.

    Args:
        version (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentAttackVersionDetail | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    version: str,
    *,
    client: AuthenticatedClient | Client,
) -> ContentAttackVersionDetail | Problem | None:
    """Read one installed ATT&CK version and family counts.

     Any authenticated subject. Exact match on the version path param —
    `15.1` does not resolve `v15.1`. Unknown version → `404`.

    Args:
        version (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentAttackVersionDetail | Problem
    """

    return (
        await asyncio_detailed(
            version=version,
            client=client,
        )
    ).parsed
