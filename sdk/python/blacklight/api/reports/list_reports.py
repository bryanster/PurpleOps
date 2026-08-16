from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.report import Report
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/reports".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | list[Report] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = Report.from_dict(response_200_item_data)

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
) -> Response[Problem | list[Report]]:
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
) -> Response[Problem | list[Report]]:
    """List reports for an engagement.

     Members and platform administrators. Returns every report in the
    engagement.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Report]]
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
) -> Problem | list[Report] | None:
    """List reports for an engagement.

     Members and platform administrators. Returns every report in the
    engagement.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Report]
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[Report]]:
    """List reports for an engagement.

     Members and platform administrators. Returns every report in the
    engagement.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Report]]
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
) -> Problem | list[Report] | None:
    """List reports for an engagement.

     Members and platform administrators. Returns every report in the
    engagement.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Report]
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
        )
    ).parsed
