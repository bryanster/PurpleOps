from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.analytics_compare import AnalyticsCompare
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    baseline: UUID,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_baseline = str(baseline)
    params["baseline"] = json_baseline

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/analytics/compare".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AnalyticsCompare | Problem | None:
    if response.status_code == 200:
        response_200 = AnalyticsCompare.from_dict(response.json())

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
) -> Response[AnalyticsCompare | Problem]:
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
    baseline: UUID,
) -> Response[AnalyticsCompare | Problem]:
    """Cross-engagement technique-by-technique comparison.

     Compare current engagement against baseline. Caller must hold report.read
    on both. Techniques matched on (technique_id, subtechnique_id) then
    template_id. No baseline_engagement_id column — any two readable
    engagements can be compared.

    Args:
        engagement_id (UUID):
        baseline (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsCompare | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        baseline=baseline,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    baseline: UUID,
) -> AnalyticsCompare | Problem | None:
    """Cross-engagement technique-by-technique comparison.

     Compare current engagement against baseline. Caller must hold report.read
    on both. Techniques matched on (technique_id, subtechnique_id) then
    template_id. No baseline_engagement_id column — any two readable
    engagements can be compared.

    Args:
        engagement_id (UUID):
        baseline (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsCompare | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        baseline=baseline,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    baseline: UUID,
) -> Response[AnalyticsCompare | Problem]:
    """Cross-engagement technique-by-technique comparison.

     Compare current engagement against baseline. Caller must hold report.read
    on both. Techniques matched on (technique_id, subtechnique_id) then
    template_id. No baseline_engagement_id column — any two readable
    engagements can be compared.

    Args:
        engagement_id (UUID):
        baseline (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AnalyticsCompare | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        baseline=baseline,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    baseline: UUID,
) -> AnalyticsCompare | Problem | None:
    """Cross-engagement technique-by-technique comparison.

     Compare current engagement against baseline. Caller must hold report.read
    on both. Techniques matched on (technique_id, subtechnique_id) then
    template_id. No baseline_engagement_id column — any two readable
    engagements can be compared.

    Args:
        engagement_id (UUID):
        baseline (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AnalyticsCompare | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            baseline=baseline,
        )
    ).parsed
