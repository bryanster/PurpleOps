from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.evidence import Evidence
from ...models.new_evidence_request import NewEvidenceRequest
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    execution_id: UUID,
    *,
    body: NewEvidenceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/executions/{execution_id}/evidence".format(
            execution_id=quote(str(execution_id), safe=""),
        ),
    }

    _kwargs["files"] = body.to_multipart()

    headers["Content-Type"] = "multipart/form-data; boundary=+++"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Evidence | Problem | None:
    if response.status_code == 201:
        response_201 = Evidence.from_dict(response.json())

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

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Evidence | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NewEvidenceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Evidence | Problem]:
    """Upload evidence to an execution.

     Lead, red, blue and platform administrators. Observer cannot upload.
    Domain enforces side: red seat → side=red only; blue → blue; lead →
    either; admin without seat may write either for support.

    File is streamed to a temp file and content-addressed by SHA-256:
    identical bytes uploaded twice produce one blob on disk with
    ref_count=2. MIME is validated against the configured allowlist.
    Default limits: 25 MiB per file, 2 GiB per engagement.

    Closed engagements return 409. Activity: `evidence.uploaded`.

    Args:
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (NewEvidenceRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Evidence | Problem]
    """

    kwargs = _get_kwargs(
        execution_id=execution_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NewEvidenceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Evidence | Problem | None:
    """Upload evidence to an execution.

     Lead, red, blue and platform administrators. Observer cannot upload.
    Domain enforces side: red seat → side=red only; blue → blue; lead →
    either; admin without seat may write either for support.

    File is streamed to a temp file and content-addressed by SHA-256:
    identical bytes uploaded twice produce one blob on disk with
    ref_count=2. MIME is validated against the configured allowlist.
    Default limits: 25 MiB per file, 2 GiB per engagement.

    Closed engagements return 409. Activity: `evidence.uploaded`.

    Args:
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (NewEvidenceRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Evidence | Problem
    """

    return sync_detailed(
        execution_id=execution_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NewEvidenceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Evidence | Problem]:
    """Upload evidence to an execution.

     Lead, red, blue and platform administrators. Observer cannot upload.
    Domain enforces side: red seat → side=red only; blue → blue; lead →
    either; admin without seat may write either for support.

    File is streamed to a temp file and content-addressed by SHA-256:
    identical bytes uploaded twice produce one blob on disk with
    ref_count=2. MIME is validated against the configured allowlist.
    Default limits: 25 MiB per file, 2 GiB per engagement.

    Closed engagements return 409. Activity: `evidence.uploaded`.

    Args:
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (NewEvidenceRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Evidence | Problem]
    """

    kwargs = _get_kwargs(
        execution_id=execution_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    execution_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NewEvidenceRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Evidence | Problem | None:
    """Upload evidence to an execution.

     Lead, red, blue and platform administrators. Observer cannot upload.
    Domain enforces side: red seat → side=red only; blue → blue; lead →
    either; admin without seat may write either for support.

    File is streamed to a temp file and content-addressed by SHA-256:
    identical bytes uploaded twice produce one blob on disk with
    ref_count=2. MIME is validated against the configured allowlist.
    Default limits: 25 MiB per file, 2 GiB per engagement.

    Closed engagements return 409. Activity: `evidence.uploaded`.

    Args:
        execution_id (UUID):
        x_csrf_token (str | Unset):
        body (NewEvidenceRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Evidence | Problem
    """

    return (
        await asyncio_detailed(
            execution_id=execution_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
