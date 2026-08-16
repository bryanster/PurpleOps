from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.add_member import AddMember
from ...models.engagement_member import EngagementMember
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    body: AddMember,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/engagements/{engagement_id}/members".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> EngagementMember | Problem | None:
    if response.status_code == 201:
        response_201 = EngagementMember.from_dict(response.json())

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
) -> Response[EngagementMember | Problem]:
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
    body: AddMember,
) -> Response[EngagementMember | Problem]:
    """Add a user to an engagement.

     Lead and platform administrators. Only `active` users may be added.
    Adding someone who is already a member returns 409. Use PATCH to
    change a member's role.

    Args:
        engagement_id (UUID):
        body (AddMember): Request to seat a user in an engagement.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EngagementMember | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AddMember,
) -> EngagementMember | Problem | None:
    """Add a user to an engagement.

     Lead and platform administrators. Only `active` users may be added.
    Adding someone who is already a member returns 409. Use PATCH to
    change a member's role.

    Args:
        engagement_id (UUID):
        body (AddMember): Request to seat a user in an engagement.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EngagementMember | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AddMember,
) -> Response[EngagementMember | Problem]:
    """Add a user to an engagement.

     Lead and platform administrators. Only `active` users may be added.
    Adding someone who is already a member returns 409. Use PATCH to
    change a member's role.

    Args:
        engagement_id (UUID):
        body (AddMember): Request to seat a user in an engagement.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EngagementMember | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AddMember,
) -> EngagementMember | Problem | None:
    """Add a user to an engagement.

     Lead and platform administrators. Only `active` users may be added.
    Adding someone who is already a member returns 409. Use PATCH to
    change a member's role.

    Args:
        engagement_id (UUID):
        body (AddMember): Request to seat a user in an engagement.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EngagementMember | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            body=body,
        )
    ).parsed
