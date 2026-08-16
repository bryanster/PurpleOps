from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...types import File, FileTypes
from io import BytesIO
from typing import cast
from uuid import UUID


def _get_kwargs(
    evidence_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/evidence/{evidence_id}/content".format(
            evidence_id=quote(str(evidence_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> File | Problem | None:
    if response.status_code == 200:
        response_200 = File(payload=BytesIO(response.content))

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[File | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    evidence_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[File | Problem]:
    """Download evidence file content.

     Any engagement member who can read the owning execution. Content-Type
    is the stored MIME from upload (allowlisted). Content-Disposition:
    attachment with filename. X-Content-Type-Options: nosniff.

    Args:
        evidence_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        evidence_id=evidence_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    evidence_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> File | Problem | None:
    """Download evidence file content.

     Any engagement member who can read the owning execution. Content-Type
    is the stored MIME from upload (allowlisted). Content-Disposition:
    attachment with filename. X-Content-Type-Options: nosniff.

    Args:
        evidence_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return sync_detailed(
        evidence_id=evidence_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    evidence_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[File | Problem]:
    """Download evidence file content.

     Any engagement member who can read the owning execution. Content-Type
    is the stored MIME from upload (allowlisted). Content-Disposition:
    attachment with filename. X-Content-Type-Options: nosniff.

    Args:
        evidence_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        evidence_id=evidence_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    evidence_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> File | Problem | None:
    """Download evidence file content.

     Any engagement member who can read the owning execution. Content-Type
    is the stored MIME from upload (allowlisted). Content-Disposition:
    attachment with filename. X-Content-Type-Options: nosniff.

    Args:
        evidence_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return (
        await asyncio_detailed(
            evidence_id=evidence_id,
            client=client,
        )
    ).parsed
