from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.report_branding_logo import ReportBrandingLogo
from ...models.upload_report_branding_logo_body import UploadReportBrandingLogoBody
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: UploadReportBrandingLogoBody,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/settings/report-branding/logo",
    }

    _kwargs["files"] = body.to_multipart()

    headers["Content-Type"] = "multipart/form-data; boundary=+++"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ReportBrandingLogo | None:
    if response.status_code == 201:
        response_201 = ReportBrandingLogo.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | ReportBrandingLogo]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: UploadReportBrandingLogoBody,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportBrandingLogo]:
    """Upload a logo for install-wide report branding.

     Administrators only. Accepts a single image file — PNG, JPEG, or WebP,
    up to 2 MiB. Returns a content-addressed blob reference suitable for
    setting in the branding defaults or per-report overrides.

    The logo is stored content-addressed under the branding directory. The
    returned blob reference is the SHA-256 hex digest of the file content.

    Args:
        x_csrf_token (str | Unset):
        body (UploadReportBrandingLogoBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportBrandingLogo]
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
    body: UploadReportBrandingLogoBody,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportBrandingLogo | None:
    """Upload a logo for install-wide report branding.

     Administrators only. Accepts a single image file — PNG, JPEG, or WebP,
    up to 2 MiB. Returns a content-addressed blob reference suitable for
    setting in the branding defaults or per-report overrides.

    The logo is stored content-addressed under the branding directory. The
    returned blob reference is the SHA-256 hex digest of the file content.

    Args:
        x_csrf_token (str | Unset):
        body (UploadReportBrandingLogoBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportBrandingLogo
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: UploadReportBrandingLogoBody,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | ReportBrandingLogo]:
    """Upload a logo for install-wide report branding.

     Administrators only. Accepts a single image file — PNG, JPEG, or WebP,
    up to 2 MiB. Returns a content-addressed blob reference suitable for
    setting in the branding defaults or per-report overrides.

    The logo is stored content-addressed under the branding directory. The
    returned blob reference is the SHA-256 hex digest of the file content.

    Args:
        x_csrf_token (str | Unset):
        body (UploadReportBrandingLogoBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportBrandingLogo]
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
    body: UploadReportBrandingLogoBody,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | ReportBrandingLogo | None:
    """Upload a logo for install-wide report branding.

     Administrators only. Accepts a single image file — PNG, JPEG, or WebP,
    up to 2 MiB. Returns a content-addressed blob reference suitable for
    setting in the branding defaults or per-report overrides.

    The logo is stored content-addressed under the branding directory. The
    returned blob reference is the SHA-256 hex digest of the file content.

    Args:
        x_csrf_token (str | Unset):
        body (UploadReportBrandingLogoBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportBrandingLogo
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
