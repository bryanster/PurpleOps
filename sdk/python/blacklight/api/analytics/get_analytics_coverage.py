from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.analytics_coverage import AnalyticsCoverage
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/analytics/coverage".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AnalyticsCoverage | Problem | None:
    if response.status_code == 200:
        response_200 = AnalyticsCoverage.from_dict(response.json())

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
) -> Response[AnalyticsCoverage | Problem]:
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
) -> Response[AnalyticsCoverage | Problem]:
    """Return technique tactic coverage for engagement.

     Both denominators — attempted (headline) matrix (pinned ATT&CK version) —
    are named fields. No bare percentage. Sub-techniques their own cells;
    parent technique counts covered only if step names it directly.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsCoverage | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> AnalyticsCoverage | Problem | None:
    """Return technique tactic coverage for engagement.

     Both denominators — attempted (headline) matrix (pinned ATT&CK version) —
    are named fields. No bare percentage. Sub-techniques their own cells;
    parent technique counts covered only if step names it directly.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsCoverage | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[AnalyticsCoverage | Problem]:
    """Return technique tactic coverage for engagement.

     Both denominators — attempted (headline) matrix (pinned ATT&CK version) —
    are named fields. No bare percentage. Sub-techniques their own cells;
    parent technique counts covered only if step names it directly.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsCoverage | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> AnalyticsCoverage | Problem | None:
    """Return technique tactic coverage for engagement.

     Both denominators — attempted (headline) matrix (pinned ATT&CK version) —
    are named fields. No bare percentage. Sub-techniques their own cells;
    parent technique counts covered only if step names it directly.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsCoverage | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
        )
    ).parsed
