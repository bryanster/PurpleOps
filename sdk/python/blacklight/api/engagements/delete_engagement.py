from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/engagements/{engagement_id}".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
    }

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
) -> Response[Any | Problem]:
    """Delete an engagement and every row in its workbook graph.

     Lead and platform administrators. Hard delete: scenarios, steps,
    executions, findings, evidence links, comments, members,
    engagement-scoped activity, finding status history, and the
    engagement's reports and report templates — including published
    versions and any share links and grants issued against them — are all
    removed. Evidence blobs have their reference released, so a blob
    nothing else points at becomes collectable.

    Not atomic: the graph is emptied in committed steps, because DuckDB
    will not let one transaction delete a row and then the row it
    references. A delete that fails partway leaves the engagement partly
    emptied and can be retried, which resumes it.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
) -> Any | Problem | None:
    """Delete an engagement and every row in its workbook graph.

     Lead and platform administrators. Hard delete: scenarios, steps,
    executions, findings, evidence links, comments, members,
    engagement-scoped activity, finding status history, and the
    engagement's reports and report templates — including published
    versions and any share links and grants issued against them — are all
    removed. Evidence blobs have their reference released, so a blob
    nothing else points at becomes collectable.

    Not atomic: the graph is emptied in committed steps, because DuckDB
    will not let one transaction delete a row and then the row it
    references. A delete that fails partway leaves the engagement partly
    emptied and can be retried, which resumes it.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Any | Problem]:
    """Delete an engagement and every row in its workbook graph.

     Lead and platform administrators. Hard delete: scenarios, steps,
    executions, findings, evidence links, comments, members,
    engagement-scoped activity, finding status history, and the
    engagement's reports and report templates — including published
    versions and any share links and grants issued against them — are all
    removed. Evidence blobs have their reference released, so a blob
    nothing else points at becomes collectable.

    Not atomic: the graph is emptied in committed steps, because DuckDB
    will not let one transaction delete a row and then the row it
    references. A delete that fails partway leaves the engagement partly
    emptied and can be retried, which resumes it.

    Args:
        engagement_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
) -> Any | Problem | None:
    """Delete an engagement and every row in its workbook graph.

     Lead and platform administrators. Hard delete: scenarios, steps,
    executions, findings, evidence links, comments, members,
    engagement-scoped activity, finding status history, and the
    engagement's reports and report templates — including published
    versions and any share links and grants issued against them — are all
    removed. Evidence blobs have their reference released, so a blob
    nothing else points at becomes collectable.

    Not atomic: the graph is emptied in committed steps, because DuckDB
    will not let one transaction delete a row and then the row it
    references. A delete that fails partway leaves the engagement partly
    emptied and can be retried, which resumes it.

    Args:
        engagement_id (UUID):

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
        )
    ).parsed
