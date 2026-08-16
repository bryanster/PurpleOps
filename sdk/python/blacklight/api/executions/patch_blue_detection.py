from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.blue_detection_patch import BlueDetectionPatch
from ...models.execution import Execution
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    execution_id: UUID,
    *,
    body: BlueDetectionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/engagements/{engagement_id}/executions/{execution_id}/detection".format(
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
    body: BlueDetectionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Execution | Problem]:
    """Write the blue (detection) side of one execution.

     Lead, blue and platform administrators. The PATCH body only accepts
    detection fields — category, modifiers, protection, timestamps,
    source/rule ref, severity and notes — so a red client cannot send
    fields it does not own. Optimistic locking: `version` is required
    and must match; mismatch → 409.

    `detected_at` before `started_at` (when both are set) → 400.
    Unreported fields are left unchanged. Unknown modifier or category
    → 400.

    A closed engagement returns 409. `scored_by` and `scored_at` are
    set when detection category or protection changes (on any successful
    patch).

    Activity: `execution.blue_updated`.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (BlueDetectionPatch): Blue-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Red fields
            are not present — red writes through a separate endpoint with
            its own type.

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
    body: BlueDetectionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Execution | Problem | None:
    """Write the blue (detection) side of one execution.

     Lead, blue and platform administrators. The PATCH body only accepts
    detection fields — category, modifiers, protection, timestamps,
    source/rule ref, severity and notes — so a red client cannot send
    fields it does not own. Optimistic locking: `version` is required
    and must match; mismatch → 409.

    `detected_at` before `started_at` (when both are set) → 400.
    Unreported fields are left unchanged. Unknown modifier or category
    → 400.

    A closed engagement returns 409. `scored_by` and `scored_at` are
    set when detection category or protection changes (on any successful
    patch).

    Activity: `execution.blue_updated`.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (BlueDetectionPatch): Blue-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Red fields
            are not present — red writes through a separate endpoint with
            its own type.

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
    body: BlueDetectionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Execution | Problem]:
    """Write the blue (detection) side of one execution.

     Lead, blue and platform administrators. The PATCH body only accepts
    detection fields — category, modifiers, protection, timestamps,
    source/rule ref, severity and notes — so a red client cannot send
    fields it does not own. Optimistic locking: `version` is required
    and must match; mismatch → 409.

    `detected_at` before `started_at` (when both are set) → 400.
    Unreported fields are left unchanged. Unknown modifier or category
    → 400.

    A closed engagement returns 409. `scored_by` and `scored_at` are
    set when detection category or protection changes (on any successful
    patch).

    Activity: `execution.blue_updated`.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (BlueDetectionPatch): Blue-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Red fields
            are not present — red writes through a separate endpoint with
            its own type.

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
    body: BlueDetectionPatch,
    x_csrf_token: str | Unset = UNSET,
) -> Execution | Problem | None:
    """Write the blue (detection) side of one execution.

     Lead, blue and platform administrators. The PATCH body only accepts
    detection fields — category, modifiers, protection, timestamps,
    source/rule ref, severity and notes — so a red client cannot send
    fields it does not own. Optimistic locking: `version` is required
    and must match; mismatch → 409.

    `detected_at` before `started_at` (when both are set) → 400.
    Unreported fields are left unchanged. Unknown modifier or category
    → 400.

    A closed engagement returns 409. `scored_by` and `scored_at` are
    set when detection category or protection changes (on any successful
    patch).

    Activity: `execution.blue_updated`.

    Args:
        engagement_id (UUID):
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (BlueDetectionPatch): Blue-side only PATCH body for an execution. `version` is the
            optimistic-lock field and is required on every call. Red fields
            are not present — red writes through a separate endpoint with
            its own type.

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
