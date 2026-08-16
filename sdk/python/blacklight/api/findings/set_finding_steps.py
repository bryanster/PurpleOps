from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.finding_step_ids import FindingStepIds
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    finding_id: UUID,
    *,
    body: FindingStepIds,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/findings/{finding_id}/steps".format(
            finding_id=quote(str(finding_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

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
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FindingStepIds,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Replace the set of steps linked to a finding.

     Lead, red or blue may set the steps linked to this finding. The
    payload is the complete set of step ids; any step not listed is
    unlinked. All step ids must belong to the same engagement as the
    finding.
    Activity: `finding.steps_changed`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (FindingStepIds):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        finding_id=finding_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FindingStepIds,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Replace the set of steps linked to a finding.

     Lead, red or blue may set the steps linked to this finding. The
    payload is the complete set of step ids; any step not listed is
    unlinked. All step ids must belong to the same engagement as the
    finding.
    Activity: `finding.steps_changed`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (FindingStepIds):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        finding_id=finding_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FindingStepIds,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Replace the set of steps linked to a finding.

     Lead, red or blue may set the steps linked to this finding. The
    payload is the complete set of step ids; any step not listed is
    unlinked. All step ids must belong to the same engagement as the
    finding.
    Activity: `finding.steps_changed`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (FindingStepIds):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        finding_id=finding_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    finding_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FindingStepIds,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Replace the set of steps linked to a finding.

     Lead, red or blue may set the steps linked to this finding. The
    payload is the complete set of step ids; any step not listed is
    unlinked. All step ids must belong to the same engagement as the
    finding.
    Activity: `finding.steps_changed`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (FindingStepIds):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            finding_id=finding_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
