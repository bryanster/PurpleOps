from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_software_list import ContentSoftwareList
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["version"] = version

    params["q"] = q

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/software",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSoftwareList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSoftwareList.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContentSoftwareList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentSoftwareList | Problem]:
    """List ATT&CK software (malware and tools).

    Args:
        version (str | Unset):
        q (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSoftwareList | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        q=q,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentSoftwareList | Problem | None:
    """List ATT&CK software (malware and tools).

    Args:
        version (str | Unset):
        q (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSoftwareList | Problem
    """

    return sync_detailed(
        client=client,
        version=version,
        q=q,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentSoftwareList | Problem]:
    """List ATT&CK software (malware and tools).

    Args:
        version (str | Unset):
        q (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSoftwareList | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        q=q,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentSoftwareList | Problem | None:
    """List ATT&CK software (malware and tools).

    Args:
        version (str | Unset):
        q (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSoftwareList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            version=version,
            q=q,
            limit=limit,
        )
    ).parsed
