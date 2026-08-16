from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    engagement_id: UUID,
    report_id: UUID,
    *,
    include_evidence: bool | Unset = True,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    params: dict[str, Any] = {}

    params["includeEvidence"] = include_evidence

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/engagements/{engagement_id}/reports/{report_id}/preview".format(
            engagement_id=quote(str(engagement_id), safe=""),
            report_id=quote(str(report_id), safe=""),
        ),
        "params": params,
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | str | None:
    if response.status_code == 200:
        response_200 = response.text
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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | str]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    include_evidence: bool | Unset = True,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | str]:
    """Render the draft report as HTML.

     Members and platform administrators. Renders the current draft blocks
    into a self-contained HTML document. The output is the same rendering
    path used for published versions, share views, and PDF input.

    Seat-scoped for blind engagements: blue members see a labelled partial
    view.

    Args:
        engagement_id (UUID):
        report_id (UUID):
        include_evidence (bool | Unset):  Default: True.
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        report_id=report_id,
        include_evidence=include_evidence,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    include_evidence: bool | Unset = True,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | str | None:
    """Render the draft report as HTML.

     Members and platform administrators. Renders the current draft blocks
    into a self-contained HTML document. The output is the same rendering
    path used for published versions, share views, and PDF input.

    Seat-scoped for blind engagements: blue members see a labelled partial
    view.

    Args:
        engagement_id (UUID):
        report_id (UUID):
        include_evidence (bool | Unset):  Default: True.
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return sync_detailed(
        engagement_id=engagement_id,
        report_id=report_id,
        client=client,
        include_evidence=include_evidence,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    include_evidence: bool | Unset = True,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | str]:
    """Render the draft report as HTML.

     Members and platform administrators. Renders the current draft blocks
    into a self-contained HTML document. The output is the same rendering
    path used for published versions, share views, and PDF input.

    Seat-scoped for blind engagements: blue members see a labelled partial
    view.

    Args:
        engagement_id (UUID):
        report_id (UUID):
        include_evidence (bool | Unset):  Default: True.
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        engagement_id=engagement_id,
        report_id=report_id,
        include_evidence=include_evidence,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    engagement_id: UUID,
    report_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    include_evidence: bool | Unset = True,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | str | None:
    """Render the draft report as HTML.

     Members and platform administrators. Renders the current draft blocks
    into a self-contained HTML document. The output is the same rendering
    path used for published versions, share views, and PDF input.

    Seat-scoped for blind engagements: blue members see a labelled partial
    view.

    Args:
        engagement_id (UUID):
        report_id (UUID):
        include_evidence (bool | Unset):  Default: True.
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return (
        await asyncio_detailed(
            engagement_id=engagement_id,
            report_id=report_id,
            client=client,
            include_evidence=include_evidence,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
