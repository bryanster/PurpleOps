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
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    presence_id: UUID | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    params: dict[str, Any] = {}

    json_presence_id: str | Unset = UNSET
    if not isinstance(presence_id, Unset):
        json_presence_id = str(presence_id)
    params["presenceId"] = json_presence_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/engagements/{engagement_id}/presence".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Any | Problem]:
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
    presence_id: UUID | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Remove this user's presence from an engagement.

     Explicitly leave presence for the authenticated session. The server
    publishes a `presence.leave` event on `engagement.{engagementId}`.
    A missing `presenceId` removes every entry for this user in this
    engagement (last tab closed).

    TTL eviction is the source of truth; this is best-effort cleanup.
    Clients SHOULD call this on tab close via `navigator.sendBeacon`.

    Args:
        engagement_id (UUID):
        presence_id (UUID | Unset):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        presence_id=presence_id,
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
    presence_id: UUID | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Remove this user's presence from an engagement.

     Explicitly leave presence for the authenticated session. The server
    publishes a `presence.leave` event on `engagement.{engagementId}`.
    A missing `presenceId` removes every entry for this user in this
    engagement (last tab closed).

    TTL eviction is the source of truth; this is best-effort cleanup.
    Clients SHOULD call this on tab close via `navigator.sendBeacon`.

    Args:
        engagement_id (UUID):
        presence_id (UUID | Unset):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        presence_id=presence_id,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    presence_id: UUID | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Remove this user's presence from an engagement.

     Explicitly leave presence for the authenticated session. The server
    publishes a `presence.leave` event on `engagement.{engagementId}`.
    A missing `presenceId` removes every entry for this user in this
    engagement (last tab closed).

    TTL eviction is the source of truth; this is best-effort cleanup.
    Clients SHOULD call this on tab close via `navigator.sendBeacon`.

    Args:
        engagement_id (UUID):
        presence_id (UUID | Unset):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        presence_id=presence_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    presence_id: UUID | Unset = UNSET,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Remove this user's presence from an engagement.

     Explicitly leave presence for the authenticated session. The server
    publishes a `presence.leave` event on `engagement.{engagementId}`.
    A missing `presenceId` removes every entry for this user in this
    engagement (last tab closed).

    TTL eviction is the source of truth; this is best-effort cleanup.
    Clients SHOULD call this on tab close via `navigator.sendBeacon`.

    Args:
        engagement_id (UUID):
        presence_id (UUID | Unset):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            presence_id=presence_id,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
