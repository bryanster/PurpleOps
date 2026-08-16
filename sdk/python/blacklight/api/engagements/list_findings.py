from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.finding import Finding
from ...models.finding_severity import FindingSeverity
from ...models.finding_status import FindingStatus
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    *,
    status: FindingStatus | Unset = UNSET,
    severity: FindingSeverity | Unset = UNSET,
    owner: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    json_severity: str | Unset = UNSET
    if not isinstance(severity, Unset):
        json_severity = severity.value

    params["severity"] = json_severity

    params["owner"] = owner

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/engagements/{engagement_id}/findings".format(
            engagement_id=quote(str(engagement_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | list[Finding] | None:
    if response.status_code == 200:
        response_200 = []
        _response_200 = response.json()
        for response_200_item_data in _response_200:
            response_200_item = Finding.from_dict(response_200_item_data)

            response_200.append(response_200_item)

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
) -> Response[Problem | list[Finding]]:
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
    status: FindingStatus | Unset = UNSET,
    severity: FindingSeverity | Unset = UNSET,
    owner: str | Unset = UNSET,
) -> Response[Problem | list[Finding]]:
    """List findings for an engagement.

     Returns every finding in this engagement, newest first. Any engagement
    member may read findings. Filters: status, severity, owner.

    Args:
        engagement_id (UUID):
        status (FindingStatus | Unset): Lifecycle of a remediation finding.
        severity (FindingSeverity | Unset): Severity of a finding.
        owner (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Finding]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        status=status,
        severity=severity,
        owner=owner,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    status: FindingStatus | Unset = UNSET,
    severity: FindingSeverity | Unset = UNSET,
    owner: str | Unset = UNSET,
) -> Problem | list[Finding] | None:
    """List findings for an engagement.

     Returns every finding in this engagement, newest first. Any engagement
    member may read findings. Filters: status, severity, owner.

    Args:
        engagement_id (UUID):
        status (FindingStatus | Unset): Lifecycle of a remediation finding.
        severity (FindingSeverity | Unset): Severity of a finding.
        owner (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Finding]
    """

    return sync_detailed(
        engagement_id=engagement_id,
        client=client,
        status=status,
        severity=severity,
        owner=owner,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    status: FindingStatus | Unset = UNSET,
    severity: FindingSeverity | Unset = UNSET,
    owner: str | Unset = UNSET,
) -> Response[Problem | list[Finding]]:
    """List findings for an engagement.

     Returns every finding in this engagement, newest first. Any engagement
    member may read findings. Filters: status, severity, owner.

    Args:
        engagement_id (UUID):
        status (FindingStatus | Unset): Lifecycle of a remediation finding.
        severity (FindingSeverity | Unset): Severity of a finding.
        owner (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | list[Finding]]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        status=status,
        severity=severity,
        owner=owner,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    status: FindingStatus | Unset = UNSET,
    severity: FindingSeverity | Unset = UNSET,
    owner: str | Unset = UNSET,
) -> Problem | list[Finding] | None:
    """List findings for an engagement.

     Returns every finding in this engagement, newest first. Any engagement
    member may read findings. Filters: status, severity, owner.

    Args:
        engagement_id (UUID):
        status (FindingStatus | Unset): Lifecycle of a remediation finding.
        severity (FindingSeverity | Unset): Severity of a finding.
        owner (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | list[Finding]
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            client=client,
            status=status,
            severity=severity,
            owner=owner,
        )
    ).parsed
