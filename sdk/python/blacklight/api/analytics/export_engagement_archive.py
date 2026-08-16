from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...types import File, FileTypes
from io import BytesIO
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/archive".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> File | Problem | None:
    if response.status_code == 200:
        response_200 = File(payload=BytesIO(response.content))

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[File | Problem]:
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
) -> Response[File | Problem]:
    """Export engagement archive as a versioned ZIP bundle.

     One versioned, self-contained archive of an engagement: structure, scores,
    findings, comments, activity and evidence files. The response is a streamed
    ZIP (Content-Type: application/zip). Format is documented in docs/archive.md.

    The archive carries a formatVersion integer at the top of manifest.json;
    a reader that does not recognise it must bail before parsing anything else.

    Blind scoping: a blue caller in a blind engagement receives a partial
    archive without unrevealed steps, their executions, evidence, or activity.
    The manifest records blindFiltered: true.

    Export only — there is no import endpoint in v1.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> File | Problem | None:
    """Export engagement archive as a versioned ZIP bundle.

     One versioned, self-contained archive of an engagement: structure, scores,
    findings, comments, activity and evidence files. The response is a streamed
    ZIP (Content-Type: application/zip). Format is documented in docs/archive.md.

    The archive carries a formatVersion integer at the top of manifest.json;
    a reader that does not recognise it must bail before parsing anything else.

    Blind scoping: a blue caller in a blind engagement receives a partial
    archive without unrevealed steps, their executions, evidence, or activity.
    The manifest records blindFiltered: true.

    Export only — there is no import endpoint in v1.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[File | Problem]:
    """Export engagement archive as a versioned ZIP bundle.

     One versioned, self-contained archive of an engagement: structure, scores,
    findings, comments, activity and evidence files. The response is a streamed
    ZIP (Content-Type: application/zip). Format is documented in docs/archive.md.

    The archive carries a formatVersion integer at the top of manifest.json;
    a reader that does not recognise it must bail before parsing anything else.

    Blind scoping: a blue caller in a blind engagement receives a partial
    archive without unrevealed steps, their executions, evidence, or activity.
    The manifest records blindFiltered: true.

    Export only — there is no import endpoint in v1.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> File | Problem | None:
    """Export engagement archive as a versioned ZIP bundle.

     One versioned, self-contained archive of an engagement: structure, scores,
    findings, comments, activity and evidence files. The response is a streamed
    ZIP (Content-Type: application/zip). Format is documented in docs/archive.md.

    The archive carries a formatVersion integer at the top of manifest.json;
    a reader that does not recognise it must bail before parsing anything else.

    Blind scoping: a blue caller in a blind engagement receives a partial
    archive without unrevealed steps, their executions, evidence, or activity.
    The manifest records blindFiltered: true.

    Export only — there is no import endpoint in v1.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
        )
    ).parsed
