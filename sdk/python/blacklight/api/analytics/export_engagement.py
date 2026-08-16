from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.export_engagement_dataset import ExportEngagementDataset
from ...models.export_engagement_format import ExportEngagementFormat
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    format_: ExportEngagementFormat,
    dataset: ExportEngagementDataset,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_format_ = format_.value
    params["format"] = json_format_

    json_dataset = dataset.value
    params["dataset"] = json_dataset

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/export".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | str | None:
    if response.status_code == 200:
        response_200 = response.text
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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | str]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    format_: ExportEngagementFormat,
    dataset: ExportEngagementDataset,
) -> Response[Problem | str]:
    """Export engagement data as flat JSON or CSV.

     Flat, tabular exports of the engagement workbook. One dataset per
    request; JSON and CSV share the same shape — one source, two encoders.
    The response is streamed, not buffered in memory.

    CSV columns that start with =, +, -, @, tab or CR are escaped with
    a single-quote prefix to prevent formula injection.

    Times are RFC 3339 UTC; durations are integer seconds.

    Args:
        engagement_id (UUID):
        format_ (ExportEngagementFormat):
        dataset (ExportEngagementDataset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        format_=format_,
        dataset=dataset,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    format_: ExportEngagementFormat,
    dataset: ExportEngagementDataset,
) -> Problem | str | None:
    """Export engagement data as flat JSON or CSV.

     Flat, tabular exports of the engagement workbook. One dataset per
    request; JSON and CSV share the same shape — one source, two encoders.
    The response is streamed, not buffered in memory.

    CSV columns that start with =, +, -, @, tab or CR are escaped with
    a single-quote prefix to prevent formula injection.

    Times are RFC 3339 UTC; durations are integer seconds.

    Args:
        engagement_id (UUID):
        format_ (ExportEngagementFormat):
        dataset (ExportEngagementDataset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        format_=format_,
        dataset=dataset,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    format_: ExportEngagementFormat,
    dataset: ExportEngagementDataset,
) -> Response[Problem | str]:
    """Export engagement data as flat JSON or CSV.

     Flat, tabular exports of the engagement workbook. One dataset per
    request; JSON and CSV share the same shape — one source, two encoders.
    The response is streamed, not buffered in memory.

    CSV columns that start with =, +, -, @, tab or CR are escaped with
    a single-quote prefix to prevent formula injection.

    Times are RFC 3339 UTC; durations are integer seconds.

    Args:
        engagement_id (UUID):
        format_ (ExportEngagementFormat):
        dataset (ExportEngagementDataset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        format_=format_,
        dataset=dataset,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    format_: ExportEngagementFormat,
    dataset: ExportEngagementDataset,
) -> Problem | str | None:
    """Export engagement data as flat JSON or CSV.

     Flat, tabular exports of the engagement workbook. One dataset per
    request; JSON and CSV share the same shape — one source, two encoders.
    The response is streamed, not buffered in memory.

    CSV columns that start with =, +, -, @, tab or CR are escaped with
    a single-quote prefix to prevent formula injection.

    Times are RFC 3339 UTC; durations are integer seconds.

    Args:
        engagement_id (UUID):
        format_ (ExportEngagementFormat):
        dataset (ExportEngagementDataset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            format_=format_,
            dataset=dataset,
        )
    ).parsed
