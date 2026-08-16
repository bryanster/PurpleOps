from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.share_password import SharePassword
from typing import cast


def _get_kwargs(
    token: str,
    *,
    body: SharePassword,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/report-views/{token}/password".format(
            token=quote(str(token), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Any | Problem]:
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
    body: SharePassword,
) -> Response[Any | Problem]:
    """Verify the share password and set a satisfaction cookie.

     Public-ish: requires a signed-in session. Verifies the password
    for a password-gated share and sets a short-lived cookie
    (bl_report_share) that authorizes subsequent HTML/PDF requests.

    Args:
        token (str):
        body (SharePassword):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: SharePassword,
) -> Any | Problem | None:
    """Verify the share password and set a satisfaction cookie.

     Public-ish: requires a signed-in session. Verifies the password
    for a password-gated share and sets a short-lived cookie
    (bl_report_share) that authorizes subsequent HTML/PDF requests.

    Args:
        token (str):
        body (SharePassword):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
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
    body: SharePassword,
) -> Response[Any | Problem]:
    """Verify the share password and set a satisfaction cookie.

     Public-ish: requires a signed-in session. Verifies the password
    for a password-gated share and sets a short-lived cookie
    (bl_report_share) that authorizes subsequent HTML/PDF requests.

    Args:
        token (str):
        body (SharePassword):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: SharePassword,
) -> Any | Problem | None:
    """Verify the share password and set a satisfaction cookie.

     Public-ish: requires a signed-in session. Verifies the password
    for a password-gated share and sets a short-lived cookie
    (bl_report_share) that authorizes subsequent HTML/PDF requests.

    Args:
        token (str):
        body (SharePassword):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            token=token,
            client=client,
            body=body,
        )
    ).parsed
