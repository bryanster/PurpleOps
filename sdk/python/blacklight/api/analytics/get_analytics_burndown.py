from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.analytics_burndown import AnalyticsBurndown
from ...models.burndown_interval import BurndownInterval
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    interval: BurndownInterval | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_interval: str | Unset = UNSET
    if not isinstance(interval, Unset):
        json_interval = interval.value

    params["interval"] = json_interval

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/analytics/burndown".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AnalyticsBurndown | Problem | None:
    if response.status_code == 200:
        response_200 = AnalyticsBurndown.from_dict(response.json())

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
) -> Response[AnalyticsBurndown | Problem]:
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
    interval: BurndownInterval | Unset = UNSET,
) -> Response[AnalyticsBurndown | Problem]:
    """Return findings burndown series severity snapshot.

     Burndown from finding_status_history, not activity log — retention-safe.
    severity snapshot is point-in-time view of current finding counts by severity.

    Args:
        engagement_id (UUID):
        interval (BurndownInterval | Unset): Bucket granularity for the burndown chart.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsBurndown | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        interval=interval,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    interval: BurndownInterval | Unset = UNSET,
) -> AnalyticsBurndown | Problem | None:
    """Return findings burndown series severity snapshot.

     Burndown from finding_status_history, not activity log — retention-safe.
    severity snapshot is point-in-time view of current finding counts by severity.

    Args:
        engagement_id (UUID):
        interval (BurndownInterval | Unset): Bucket granularity for the burndown chart.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsBurndown | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        interval=interval,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    interval: BurndownInterval | Unset = UNSET,
) -> Response[AnalyticsBurndown | Problem]:
    """Return findings burndown series severity snapshot.

     Burndown from finding_status_history, not activity log — retention-safe.
    severity snapshot is point-in-time view of current finding counts by severity.

    Args:
        engagement_id (UUID):
        interval (BurndownInterval | Unset): Bucket granularity for the burndown chart.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsBurndown | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        interval=interval,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    interval: BurndownInterval | Unset = UNSET,
) -> AnalyticsBurndown | Problem | None:
    """Return findings burndown series severity snapshot.

     Burndown from finding_status_history, not activity log — retention-safe.
    severity snapshot is point-in-time view of current finding counts by severity.

    Args:
        engagement_id (UUID):
        interval (BurndownInterval | Unset): Bucket granularity for the burndown chart.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsBurndown | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            interval=interval,
        )
    ).parsed
