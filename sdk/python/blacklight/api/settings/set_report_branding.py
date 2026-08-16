from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.report_branding import ReportBranding
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: ReportBranding,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/settings/report-branding",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ReportBranding | None:
    if response.status_code == 200:
        response_200 = ReportBranding.from_dict(response.json())

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
) -> Response[Problem | ReportBranding]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ReportBranding,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportBranding]:
    """Replace the install-wide report branding defaults.

     Administrators only. A whole replacement rather than a patch: all fields
    are required, so two administrators editing at once cannot each change
    the half they were thinking about and silently keep the other's.

    Args:
        x_csrf_token (str | Unset):
        body (ReportBranding): Install-wide default branding for report generation. Every field
            has a
            built-in fallback so a fresh deployment produces readable output without
            configuration. Per-report overrides (client name, logo, colours) take
            precedence over these defaults; the resolution order is defined in
            docs/reporting.md.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportBranding]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: ReportBranding,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportBranding | None:
    """Replace the install-wide report branding defaults.

     Administrators only. A whole replacement rather than a patch: all fields
    are required, so two administrators editing at once cannot each change
    the half they were thinking about and silently keep the other's.

    Args:
        x_csrf_token (str | Unset):
        body (ReportBranding): Install-wide default branding for report generation. Every field
            has a
            built-in fallback so a fresh deployment produces readable output without
            configuration. Per-report overrides (client name, logo, colours) take
            precedence over these defaults; the resolution order is defined in
            docs/reporting.md.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportBranding
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ReportBranding,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportBranding]:
    """Replace the install-wide report branding defaults.

     Administrators only. A whole replacement rather than a patch: all fields
    are required, so two administrators editing at once cannot each change
    the half they were thinking about and silently keep the other's.

    Args:
        x_csrf_token (str | Unset):
        body (ReportBranding): Install-wide default branding for report generation. Every field
            has a
            built-in fallback so a fresh deployment produces readable output without
            configuration. Per-report overrides (client name, logo, colours) take
            precedence over these defaults; the resolution order is defined in
            docs/reporting.md.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportBranding]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: ReportBranding,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportBranding | None:
    """Replace the install-wide report branding defaults.

     Administrators only. A whole replacement rather than a patch: all fields
    are required, so two administrators editing at once cannot each change
    the half they were thinking about and silently keep the other's.

    Args:
        x_csrf_token (str | Unset):
        body (ReportBranding): Install-wide default branding for report generation. Every field
            has a
            built-in fallback so a fresh deployment produces readable output without
            configuration. Per-report overrides (client name, logo, colours) take
            precedence over these defaults; the resolution order is defined in
            docs/reporting.md.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportBranding
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
