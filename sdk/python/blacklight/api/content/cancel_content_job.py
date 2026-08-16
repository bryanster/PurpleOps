from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_sync_job import ContentSyncJob
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    job_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/content/jobs/{job_id}/cancel".format(
            job_id=quote(str(job_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentSyncJob | Problem | None:
    if response.status_code == 200:
        response_200 = ContentSyncJob.from_dict(response.json())

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

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContentSyncJob | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    job_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSyncJob | Problem]:
    """Cancel a content sync job.

     Administrators only (`content.sync`). A `queued` job becomes
    `cancelled` immediately. A `running` job enters `cancelling`; adapter
    steps observe context cancel and the job ends `cancelled`. Terminal
    jobs answer `409`.

    Args:
        job_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSyncJob | Problem]
    """

    kwargs = _get_kwargs(
        job_id=job_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    job_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSyncJob | Problem | None:
    """Cancel a content sync job.

     Administrators only (`content.sync`). A `queued` job becomes
    `cancelled` immediately. A `running` job enters `cancelling`; adapter
    steps observe context cancel and the job ends `cancelled`. Terminal
    jobs answer `409`.

    Args:
        job_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSyncJob | Problem
    """

    return sync_detailed(
        job_id=job_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    job_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentSyncJob | Problem]:
    """Cancel a content sync job.

     Administrators only (`content.sync`). A `queued` job becomes
    `cancelled` immediately. A `running` job enters `cancelling`; adapter
    steps observe context cancel and the job ends `cancelled`. Terminal
    jobs answer `409`.

    Args:
        job_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentSyncJob | Problem]
    """

    kwargs = _get_kwargs(
        job_id=job_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    job_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> ContentSyncJob | Problem | None:
    """Cancel a content sync job.

     Administrators only (`content.sync`). A `queued` job becomes
    `cancelled` immediately. A `running` job enters `cancelling`; adapter
    steps observe context cancel and the job ends `cancelled`. Terminal
    jobs answer `409`.

    Args:
        job_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentSyncJob | Problem
    """

    return (
        await asyncio_detailed(
            job_id=job_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
