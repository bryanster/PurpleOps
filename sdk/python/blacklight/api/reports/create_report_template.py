from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.create_report_template import CreateReportTemplate
from ...models.problem import Problem
from ...models.report_template import ReportTemplate
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    body: CreateReportTemplate,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/engagements/{engagement_id}/report-templates".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ReportTemplate | None:
    if response.status_code == 201:
        response_201 = ReportTemplate.from_dict(response.json())

        return response_201

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
) -> Response[Problem | ReportTemplate]:
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
    body: CreateReportTemplate,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportTemplate]:
    """Create a new report template in an engagement.

     Every member of the engagement and platform administrators.

    Args:
        engagement_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportTemplate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportTemplate]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportTemplate,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportTemplate | None:
    """Create a new report template in an engagement.

     Every member of the engagement and platform administrators.

    Args:
        engagement_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportTemplate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportTemplate
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportTemplate,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportTemplate]:
    """Create a new report template in an engagement.

     Every member of the engagement and platform administrators.

    Args:
        engagement_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportTemplate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportTemplate]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CreateReportTemplate,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportTemplate | None:
    """Create a new report template in an engagement.

     Every member of the engagement and platform administrators.

    Args:
        engagement_id (UUID):
        x_csrf_token (str | Unset):
        body (CreateReportTemplate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportTemplate
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
