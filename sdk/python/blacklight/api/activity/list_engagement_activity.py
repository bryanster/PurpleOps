from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.activity_page import ActivityPage
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    actor: UUID | Unset = UNSET,
    verb: str | Unset = UNSET,
    object_type: str | Unset = UNSET,
    object_id: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["limit"] = limit

    params["cursor"] = cursor

    json_actor: str | Unset = UNSET
    if not isinstance(actor, Unset):
        json_actor = str(actor)
    params["actor"] = json_actor

    params["verb"] = verb

    params["objectType"] = object_type

    params["objectId"] = object_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/activity".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> ActivityPage | Problem | None:
    if response.status_code == 200:
        response_200 = ActivityPage.from_dict(response.json())

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
) -> Response[ActivityPage | Problem]:
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
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    actor: UUID | Unset = UNSET,
    verb: str | Unset = UNSET,
    object_type: str | Unset = UNSET,
    object_id: str | Unset = UNSET,
) -> Response[ActivityPage | Problem]:
    """List one engagement's activity log.

     Members of the engagement, and platform administrators. The rows that
    name this engagement — comments, executions, membership changes once
    those exist — newest first.

    A non-member is answered 404 rather than 403, so the existence of an
    engagement is not confirmed to someone outside it.

    Args:
        engagement_id (UUID):
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        actor (UUID | Unset):
        verb (str | Unset):
        object_type (str | Unset):
        object_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ActivityPage | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        limit=limit,
        cursor=cursor,
        actor=actor,
        verb=verb,
        object_type=object_type,
        object_id=object_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    actor: UUID | Unset = UNSET,
    verb: str | Unset = UNSET,
    object_type: str | Unset = UNSET,
    object_id: str | Unset = UNSET,
) -> ActivityPage | Problem | None:
    """List one engagement's activity log.

     Members of the engagement, and platform administrators. The rows that
    name this engagement — comments, executions, membership changes once
    those exist — newest first.

    A non-member is answered 404 rather than 403, so the existence of an
    engagement is not confirmed to someone outside it.

    Args:
        engagement_id (UUID):
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        actor (UUID | Unset):
        verb (str | Unset):
        object_type (str | Unset):
        object_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ActivityPage | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        limit=limit,
        cursor=cursor,
        actor=actor,
        verb=verb,
        object_type=object_type,
        object_id=object_id,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    actor: UUID | Unset = UNSET,
    verb: str | Unset = UNSET,
    object_type: str | Unset = UNSET,
    object_id: str | Unset = UNSET,
) -> Response[ActivityPage | Problem]:
    """List one engagement's activity log.

     Members of the engagement, and platform administrators. The rows that
    name this engagement — comments, executions, membership changes once
    those exist — newest first.

    A non-member is answered 404 rather than 403, so the existence of an
    engagement is not confirmed to someone outside it.

    Args:
        engagement_id (UUID):
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        actor (UUID | Unset):
        verb (str | Unset):
        object_type (str | Unset):
        object_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ActivityPage | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        limit=limit,
        cursor=cursor,
        actor=actor,
        verb=verb,
        object_type=object_type,
        object_id=object_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    actor: UUID | Unset = UNSET,
    verb: str | Unset = UNSET,
    object_type: str | Unset = UNSET,
    object_id: str | Unset = UNSET,
) -> ActivityPage | Problem | None:
    """List one engagement's activity log.

     Members of the engagement, and platform administrators. The rows that
    name this engagement — comments, executions, membership changes once
    those exist — newest first.

    A non-member is answered 404 rather than 403, so the existence of an
    engagement is not confirmed to someone outside it.

    Args:
        engagement_id (UUID):
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        actor (UUID | Unset):
        verb (str | Unset):
        object_type (str | Unset):
        object_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ActivityPage | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            limit=limit,
            cursor=cursor,
            actor=actor,
            verb=verb,
            object_type=object_type,
            object_id=object_id,
        )
    ).parsed
