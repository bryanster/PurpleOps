from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.comment import Comment
from ...models.patch_comment import PatchComment
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    body: PatchComment,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/engagements/{engagement_id}/comments/{comment_id}".format(
            engagement_id=quote(str(engagement_id), safe=""),
            comment_id=quote(str(comment_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Comment | Problem | None:
    if response.status_code == 200:
        response_200 = Comment.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Comment | Problem]:
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
    body: PatchComment,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Comment | Problem]:
    """Edit a comment's body.

     Only the comment's author, the engagement lead, or a platform
    administrator may edit. The previous body is appended to the revision
    history. No delete — history is append-only.

    Args:
        engagement_id (UUID):
        comment_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchComment):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Comment | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        comment_id=comment_id,
        body=body,
        x_csrf_token=x_csrf_token,
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
    body: PatchComment,
    x_csrf_token: str | Unset = UNSET,
) -> Comment | Problem | None:
    """Edit a comment's body.

     Only the comment's author, the engagement lead, or a platform
    administrator may edit. The previous body is appended to the revision
    history. No delete — history is append-only.

    Args:
        engagement_id (UUID):
        comment_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchComment):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Comment | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        comment_id=comment_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: PatchComment,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Comment | Problem]:
    """Edit a comment's body.

     Only the comment's author, the engagement lead, or a platform
    administrator may edit. The previous body is appended to the revision
    history. No delete — history is append-only.

    Args:
        engagement_id (UUID):
        comment_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchComment):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Comment | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        comment_id=comment_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    comment_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: PatchComment,
    x_csrf_token: str | Unset = UNSET,
) -> Comment | Problem | None:
    """Edit a comment's body.

     Only the comment's author, the engagement lead, or a platform
    administrator may edit. The previous body is appended to the revision
    history. No delete — history is append-only.

    Args:
        engagement_id (UUID):
        comment_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchComment):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Comment | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            comment_id=comment_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
