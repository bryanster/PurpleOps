from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.report_version import ReportVersion
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    report_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/reports/{report_id}/versions".format(
            engagement_id=quote(str(engagement_id), safe=""),
            report_id=quote(str(report_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | list[ReportVersion] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = ReportVersion.from_dict(response_200_item_data)

            response_200.append(response_200_item)

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
) -> Response[Problem | list[ReportVersion]]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[ReportVersion]]:
    """List published versions of a report.

     Members and platform administrators with report.read. Returns versions
    newest first.

    Args:
        engagement_id (UUID):
        report_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[ReportVersion]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        report_id=report_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[ReportVersion] | None:
    """List published versions of a report.

     Members and platform administrators with report.read. Returns versions
    newest first.

    Args:
        engagement_id (UUID):
        report_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[ReportVersion]
    """

    return sync_detailed(
        engagement_id=engagement_id,
        report_id=report_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[ReportVersion]]:
    """List published versions of a report.

     Members and platform administrators with report.read. Returns versions
    newest first.

    Args:
        engagement_id (UUID):
        report_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[ReportVersion]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        report_id=report_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[ReportVersion] | None:
    """List published versions of a report.

     Members and platform administrators with report.read. Returns versions
    newest first.

    Args:
        engagement_id (UUID):
        report_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[ReportVersion]
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            report_id=report_id,
            client=client,
        )
    ).parsed
