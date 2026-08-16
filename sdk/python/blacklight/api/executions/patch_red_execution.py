from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.execution import Execution
from ...models.problem import Problem
from ...models.red_execution_patch import RedExecutionPatch
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    body: RedExecutionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/engagements/{engagement_id}/executions/{execution_id}/execution".format(
            engagement_id=quote(str(engagement_id), safe=""),
            execution_id=quote(str(execution_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Execution | Problem | None:
    if response.status_code == 200:
        response_200 = Execution.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Execution | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RedExecutionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Execution | Problem]:
    """Write the red (attack) side of one execution.

     Lead, red and platform administrators. The PATCH body only accepts
    red fields — status, timestamps, command, hosts, notes — so a blue
    client cannot send fields it does not own. Optimistic locking:
    `version` is required and must match; mismatch → 409.

    Status transitions: `pending` → `running`|`skipped`|`blocked`;
    `running` → `complete`|`blocked`|`skipped`. Terminal states
    (complete/blocked/skipped) accept note/host/command edits without
    a status change. Illegal jumps → 409.

    Auto-reveal: when the status becomes `running` or `complete` and
    the engagement is blind with `auto_reveal_on_start` enabled and
    the step is unrevealed, the step is revealed in the same transaction.

    A closed engagement returns 409. The first non-pending transition
    sets `executed_by` to the caller's id if still empty.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (RedExecutionPatch): Red-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Detection
            fields are not present — blue writes through a separate endpoint
            (M3-007) with its own type.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Execution | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        execution_id=execution_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RedExecutionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Execution | Problem | None:
    """Write the red (attack) side of one execution.

     Lead, red and platform administrators. The PATCH body only accepts
    red fields — status, timestamps, command, hosts, notes — so a blue
    client cannot send fields it does not own. Optimistic locking:
    `version` is required and must match; mismatch → 409.

    Status transitions: `pending` → `running`|`skipped`|`blocked`;
    `running` → `complete`|`blocked`|`skipped`. Terminal states
    (complete/blocked/skipped) accept note/host/command edits without
    a status change. Illegal jumps → 409.

    Auto-reveal: when the status becomes `running` or `complete` and
    the engagement is blind with `auto_reveal_on_start` enabled and
    the step is unrevealed, the step is revealed in the same transaction.

    A closed engagement returns 409. The first non-pending transition
    sets `executed_by` to the caller's id if still empty.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (RedExecutionPatch): Red-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Detection
            fields are not present — blue writes through a separate endpoint
            (M3-007) with its own type.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Execution | Problem
    """

    return sync_detailed(
        engagement_id=engagement_id,
        execution_id=execution_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RedExecutionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Execution | Problem]:
    """Write the red (attack) side of one execution.

     Lead, red and platform administrators. The PATCH body only accepts
    red fields — status, timestamps, command, hosts, notes — so a blue
    client cannot send fields it does not own. Optimistic locking:
    `version` is required and must match; mismatch → 409.

    Status transitions: `pending` → `running`|`skipped`|`blocked`;
    `running` → `complete`|`blocked`|`skipped`. Terminal states
    (complete/blocked/skipped) accept note/host/command edits without
    a status change. Illegal jumps → 409.

    Auto-reveal: when the status becomes `running` or `complete` and
    the engagement is blind with `auto_reveal_on_start` enabled and
    the step is unrevealed, the step is revealed in the same transaction.

    A closed engagement returns 409. The first non-pending transition
    sets `executed_by` to the caller's id if still empty.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (RedExecutionPatch): Red-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Detection
            fields are not present — blue writes through a separate endpoint
            (M3-007) with its own type.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Execution | Problem]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        execution_id=execution_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RedExecutionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Execution | Problem | None:
    """Write the red (attack) side of one execution.

     Lead, red and platform administrators. The PATCH body only accepts
    red fields — status, timestamps, command, hosts, notes — so a blue
    client cannot send fields it does not own. Optimistic locking:
    `version` is required and must match; mismatch → 409.

    Status transitions: `pending` → `running`|`skipped`|`blocked`;
    `running` → `complete`|`blocked`|`skipped`. Terminal states
    (complete/blocked/skipped) accept note/host/command edits without
    a status change. Illegal jumps → 409.

    Auto-reveal: when the status becomes `running` or `complete` and
    the engagement is blind with `auto_reveal_on_start` enabled and
    the step is unrevealed, the step is revealed in the same transaction.

    A closed engagement returns 409. The first non-pending transition
    sets `executed_by` to the caller's id if still empty.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (RedExecutionPatch): Red-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Detection
            fields are not present — blue writes through a separate endpoint
            (M3-007) with its own type.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Execution | Problem
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            execution_id=execution_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
