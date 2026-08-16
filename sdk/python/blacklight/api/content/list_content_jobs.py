from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_sync_job_list import ContentSyncJobList
from ...models.content_sync_job_status import ContentSyncJobStatus
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    *,
    status: ContentSyncJobStatus | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 50,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    json_source_id: str | Unset = UNSET
    if not isinstance(source_id, Unset):
        json_source_id = str(source_id)
    params["sourceId"] = json_source_id

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/jobs",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSyncJobList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSyncJobList.from_dict(response.json())

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
) -> Response[ContentSyncJobList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    status: ContentSyncJobStatus | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[ContentSyncJobList | Problem]:
    """List content sync jobs.

     Administrators only (`content.sync`). Returns jobs newest first. Filter
    by `status` and/or `sourceId`. Job list is admin-only in M2 — members
    read progress via source detail's `lastJob` summary (`content.read`).

    Args:
        status (ContentSyncJobStatus | Unset): Lifecycle state of a content sync job.
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSyncJobList | Problem]
    """

    kwargs = _get_kwargs(
        status=status,
        source_id=source_id,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    status: ContentSyncJobStatus | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 50,
) -> ContentSyncJobList | Problem | None:
    """List content sync jobs.

     Administrators only (`content.sync`). Returns jobs newest first. Filter
    by `status` and/or `sourceId`. Job list is admin-only in M2 — members
    read progress via source detail's `lastJob` summary (`content.read`).

    Args:
        status (ContentSyncJobStatus | Unset): Lifecycle state of a content sync job.
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSyncJobList | Problem
    """

    return sync_detailed(
        client=client,
        status=status,
        source_id=source_id,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    status: ContentSyncJobStatus | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[ContentSyncJobList | Problem]:
    """List content sync jobs.

     Administrators only (`content.sync`). Returns jobs newest first. Filter
    by `status` and/or `sourceId`. Job list is admin-only in M2 — members
    read progress via source detail's `lastJob` summary (`content.read`).

    Args:
        status (ContentSyncJobStatus | Unset): Lifecycle state of a content sync job.
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSyncJobList | Problem]
    """

    kwargs = _get_kwargs(
        status=status,
        source_id=source_id,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    status: ContentSyncJobStatus | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 50,
) -> ContentSyncJobList | Problem | None:
    """List content sync jobs.

     Administrators only (`content.sync`). Returns jobs newest first. Filter
    by `status` and/or `sourceId`. Job list is admin-only in M2 — members
    read progress via source detail's `lastJob` summary (`content.read`).

    Args:
        status (ContentSyncJobStatus | Unset): Lifecycle state of a content sync job.
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSyncJobList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            status=status,
            source_id=source_id,
            limit=limit,
        )
    ).parsed
