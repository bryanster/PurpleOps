from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.claim_report_share import ClaimReportShare
from ...models.claim_report_share_result import ClaimReportShareResult
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    token: str,
    *,
    body: ClaimReportShare | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/report-views/{token}/claim".format(
            token=quote(str(token), safe=""),
        ),
    }

    if not isinstance(body, Unset):
        _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ClaimReportShareResult | Problem | None:
    if response.status_code == 200:
        response_200 = ClaimReportShareResult.from_dict(response.json())

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
) -> Response[ClaimReportShareResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: ClaimReportShare | Unset = UNSET,
) -> Response[ClaimReportShareResult | Problem]:
    """Claim access to a shared report version.

     Public-ish: requires a signed-in session. Binds a grant to the
    caller's user account. If the share has a password gate, the
    password must be provided.

    Args:
        token (str):
        body (ClaimReportShare | Unset): Body of POST /report-views/{token}/claim.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ClaimReportShareResult | Problem]
    """

    kwargs = _get_kwargs(
        token=token,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: ClaimReportShare | Unset = UNSET,
) -> ClaimReportShareResult | Problem | None:
    """Claim access to a shared report version.

     Public-ish: requires a signed-in session. Binds a grant to the
    caller's user account. If the share has a password gate, the
    password must be provided.

    Args:
        token (str):
        body (ClaimReportShare | Unset): Body of POST /report-views/{token}/claim.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ClaimReportShareResult | Problem
    """

    return sync_detailed(
        token=token,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: ClaimReportShare | Unset = UNSET,
) -> Response[ClaimReportShareResult | Problem]:
    """Claim access to a shared report version.

     Public-ish: requires a signed-in session. Binds a grant to the
    caller's user account. If the share has a password gate, the
    password must be provided.

    Args:
        token (str):
        body (ClaimReportShare | Unset): Body of POST /report-views/{token}/claim.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ClaimReportShareResult | Problem]
    """

    kwargs = _get_kwargs(
        token=token,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: ClaimReportShare | Unset = UNSET,
) -> ClaimReportShareResult | Problem | None:
    """Claim access to a shared report version.

     Public-ish: requires a signed-in session. Binds a grant to the
    caller's user account. If the share has a password gate, the
    password must be provided.

    Args:
        token (str):
        body (ClaimReportShare | Unset): Body of POST /report-views/{token}/claim.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ClaimReportShareResult | Problem
    """

    return (
        await asyncio_detailed(
            token=token,
            client=client,
            body=body,
        )
    ).parsed
