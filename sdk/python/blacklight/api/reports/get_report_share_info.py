from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.report_share_info import ReportShareInfo
from typing import cast


def _get_kwargs(
    token: str,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/report-views/{token}".format(
            token=quote(str(token), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ReportShareInfo | None:
    if response.status_code == 200:
        response_200 = ReportShareInfo.from_dict(response.json())

        return response_200

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
) -> Response[Problem | ReportShareInfo]:
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
) -> Response[Problem | ReportShareInfo]:
    """Return share metadata for the claim page.

     Public-ish: requires a signed-in session, but authorization is
    by share grant, not engagement membership. Returns whether the
    share exists, requires a password, and whether the caller has
    already claimed it.

    Args:
        token (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportShareInfo]
    """

    kwargs = _get_kwargs(
        token=token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    token: str,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ReportShareInfo | None:
    """Return share metadata for the claim page.

     Public-ish: requires a signed-in session, but authorization is
    by share grant, not engagement membership. Returns whether the
    share exists, requires a password, and whether the caller has
    already claimed it.

    Args:
        token (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportShareInfo
    """

    return sync_detailed(
        token=token,
        client=client,
    ).parsed


async def asyncio_detailed(
    token: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | ReportShareInfo]:
    """Return share metadata for the claim page.

     Public-ish: requires a signed-in session, but authorization is
    by share grant, not engagement membership. Returns whether the
    share exists, requires a password, and whether the caller has
    already claimed it.

    Args:
        token (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReportShareInfo]
    """

    kwargs = _get_kwargs(
        token=token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    token: str,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ReportShareInfo | None:
    """Return share metadata for the claim page.

     Public-ish: requires a signed-in session, but authorization is
    by share grant, not engagement membership. Returns whether the
    share exists, requires a password, and whether the caller has
    already claimed it.

    Args:
        token (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReportShareInfo
    """

    return (
        await asyncio_detailed(
            token=token,
            client=client,
        )
    ).parsed
