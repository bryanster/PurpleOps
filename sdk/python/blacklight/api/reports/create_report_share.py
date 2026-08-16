from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.create_report_share import CreateReportShare
from ...models.create_report_share_result import CreateReportShareResult
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    version_id: UUID,
    *,
    body: CreateReportShare | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/report-versions/{version_id}/shares".format(
            version_id=quote(str(version_id), safe=""),
        ),
    }

    if not isinstance(body, Unset):
        _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CreateReportShareResult | Problem | None:
    if response.status_code == 201:
        response_201 = CreateReportShareResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

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
) -> Response[CreateReportShareResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    version_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportShare | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Response[CreateReportShareResult | Problem]:
    """Create a share link for a published version.

     Lead only (report.publish). Creates an unguessable share link.
    The share token is returned once in the response — it is never
    stored plaintext on the server.

    Optional password gate, expiry, max grants, and label.

    Args:
        version_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportShare | Unset): Body of POST /report-versions/{versionId}/shares.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateReportShareResult | Problem]
    """

    kwargs = _get_kwargs(
        version_id=version_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    version_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportShare | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> CreateReportShareResult | Problem | None:
    """Create a share link for a published version.

     Lead only (report.publish). Creates an unguessable share link.
    The share token is returned once in the response — it is never
    stored plaintext on the server.

    Optional password gate, expiry, max grants, and label.

    Args:
        version_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportShare | Unset): Body of POST /report-versions/{versionId}/shares.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateReportShareResult | Problem
    """

    return sync_detailed(
        version_id=version_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    version_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportShare | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Response[CreateReportShareResult | Problem]:
    """Create a share link for a published version.

     Lead only (report.publish). Creates an unguessable share link.
    The share token is returned once in the response — it is never
    stored plaintext on the server.

    Optional password gate, expiry, max grants, and label.

    Args:
        version_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportShare | Unset): Body of POST /report-versions/{versionId}/shares.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateReportShareResult | Problem]
    """

    kwargs = _get_kwargs(
        version_id=version_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    version_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportShare | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> CreateReportShareResult | Problem | None:
    """Create a share link for a published version.

     Lead only (report.publish). Creates an unguessable share link.
    The share token is returned once in the response — it is never
    stored plaintext on the server.

    Optional password gate, expiry, max grants, and label.

    Args:
        version_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportShare | Unset): Body of POST /report-versions/{versionId}/shares.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateReportShareResult | Problem
    """

    return (
        await asyncio_detailed(
            version_id=version_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
