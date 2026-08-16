from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.finding import Finding
from ...models.patch_finding import PatchFinding
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    finding_id: UUID,
    *,
    body: PatchFinding,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/findings/{finding_id}".format(
            finding_id=quote(str(finding_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Finding | Problem | None:
    if response.status_code == 200:
        response_200 = Finding.from_dict(response.json())

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Finding | Problem]:
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
    body: PatchFinding,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Finding | Problem]:
    """Update a finding.

     Lead, red or blue may update a finding. Any field not sent stays
    unchanged. Closed engagements may still update the status field
    (e.g. to resolved); other fields on a closed engagement return 409.
    Activity: `finding.updated`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchFinding):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Finding | Problem]
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
    body: PatchFinding,
    x_csrf_token: str | Unset = UNSET,
) -> Finding | Problem | None:
    """Update a finding.

     Lead, red or blue may update a finding. Any field not sent stays
    unchanged. Closed engagements may still update the status field
    (e.g. to resolved); other fields on a closed engagement return 409.
    Activity: `finding.updated`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchFinding):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Finding | Problem
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
    body: PatchFinding,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Finding | Problem]:
    """Update a finding.

     Lead, red or blue may update a finding. Any field not sent stays
    unchanged. Closed engagements may still update the status field
    (e.g. to resolved); other fields on a closed engagement return 409.
    Activity: `finding.updated`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchFinding):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Finding | Problem]
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
    body: PatchFinding,
    x_csrf_token: str | Unset = UNSET,
) -> Finding | Problem | None:
    """Update a finding.

     Lead, red or blue may update a finding. Any field not sent stays
    unchanged. Closed engagements may still update the status field
    (e.g. to resolved); other fields on a closed engagement return 409.
    Activity: `finding.updated`.

    Args:
        finding_id (UUID):
        x_csrf_token (str | Unset):
        body (PatchFinding):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Finding | Problem
    """

    return (
        await asyncio_detailed(
            finding_id=finding_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
