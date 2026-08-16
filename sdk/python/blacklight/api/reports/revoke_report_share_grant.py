from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    share_id: str,
    grant_id: str,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/report-shares/{share_id}/grants/{grant_id}".format(
            share_id=quote(str(share_id), safe=""),
            grant_id=quote(str(grant_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    share_id: str,
    grant_id: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke a single grant.

     Lead only (report.publish). Revokes one grant on a share.
    The grant holder's subsequent access returns 404.

    Args:
        share_id (str):
        grant_id (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        share_id=share_id,
        grant_id=grant_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    share_id: str,
    grant_id: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke a single grant.

     Lead only (report.publish). Revokes one grant on a share.
    The grant holder's subsequent access returns 404.

    Args:
        share_id (str):
        grant_id (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        share_id=share_id,
        grant_id=grant_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    share_id: str,
    grant_id: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Revoke a single grant.

     Lead only (report.publish). Revokes one grant on a share.
    The grant holder's subsequent access returns 404.

    Args:
        share_id (str):
        grant_id (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        share_id=share_id,
        grant_id=grant_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    share_id: str,
    grant_id: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Revoke a single grant.

     Lead only (report.publish). Revokes one grant on a share.
    The grant holder's subsequent access returns 404.

    Args:
        share_id (str):
        grant_id (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            share_id=share_id,
            grant_id=grant_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
