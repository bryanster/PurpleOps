from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.comment_revision import CommentRevision
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    comment_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/comments/{comment_id}/revisions".format(
            engagement_id=quote(str(engagement_id), safe=""),
            comment_id=quote(str(comment_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | list[CommentRevision] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = CommentRevision.from_dict(response_200_item_data)

            response_200.append(response_200_item)

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | list[CommentRevision]]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[CommentRevision]]:
    """List the edit history of a comment.

     Returns every prior body of this comment, oldest first. The current
    body is not included — it is on the comment itself. An edit appends
    the previous body, so the list is complete from creation to the
    penultimate edit.

    Args:
        engagement_id (UUID):
        comment_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[CommentRevision]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        comment_id=comment_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[CommentRevision] | None:
    """List the edit history of a comment.

     Returns every prior body of this comment, oldest first. The current
    body is not included — it is on the comment itself. An edit appends
    the previous body, so the list is complete from creation to the
    penultimate edit.

    Args:
        engagement_id (UUID):
        comment_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[CommentRevision]
    """

    return sync_detailed(
        engagement_id=engagement_id,
        comment_id=comment_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | list[CommentRevision]]:
    """List the edit history of a comment.

     Returns every prior body of this comment, oldest first. The current
    body is not included — it is on the comment itself. An edit appends
    the previous body, so the list is complete from creation to the
    penultimate edit.

    Args:
        engagement_id (UUID):
        comment_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[CommentRevision]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        comment_id=comment_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | list[CommentRevision] | None:
    """List the edit history of a comment.

     Returns every prior body of this comment, oldest first. The current
    body is not included — it is on the comment itself. An edit appends
    the previous body, so the list is complete from creation to the
    penultimate edit.

    Args:
        engagement_id (UUID):
        comment_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[CommentRevision]
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            comment_id=comment_id,
            client=client,
        )
    ).parsed
