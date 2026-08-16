from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_procedure_template import ContentProcedureTemplate
from ...models.create_custom_procedure_template_request import CreateCustomProcedureTemplateRequest
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: CreateCustomProcedureTemplateRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/content/custom/procedure-templates",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentProcedureTemplate | Problem | None:
    if response.status_code == 201:
        response_201 = ContentProcedureTemplate.from_dict(response.json())

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
) -> Response[ContentProcedureTemplate | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomProcedureTemplateRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentProcedureTemplate | Problem]:
    """Create a custom procedure template.

     Administrators only (`content.manage`). Attaches to the singleton
    `custom` source with version `current`. Technique external ids are
    optional; when present they must look like MITRE ids (`T1059`,
    `T1059.001`).

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomProcedureTemplateRequest): Body for creating a custom procedure
            template.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplate | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomProcedureTemplateRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentProcedureTemplate | Problem | None:
    """Create a custom procedure template.

     Administrators only (`content.manage`). Attaches to the singleton
    `custom` source with version `current`. Technique external ids are
    optional; when present they must look like MITRE ids (`T1059`,
    `T1059.001`).

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomProcedureTemplateRequest): Body for creating a custom procedure
            template.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplate | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomProcedureTemplateRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentProcedureTemplate | Problem]:
    """Create a custom procedure template.

     Administrators only (`content.manage`). Attaches to the singleton
    `custom` source with version `current`. Technique external ids are
    optional; when present they must look like MITRE ids (`T1059`,
    `T1059.001`).

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomProcedureTemplateRequest): Body for creating a custom procedure
            template.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplate | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: CreateCustomProcedureTemplateRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentProcedureTemplate | Problem | None:
    """Create a custom procedure template.

     Administrators only (`content.manage`). Attaches to the singleton
    `custom` source with version `current`. Technique external ids are
    optional; when present they must look like MITRE ids (`T1059`,
    `T1059.001`).

    Args:
        x_csrf_token (str | Unset):
        body (CreateCustomProcedureTemplateRequest): Body for creating a custom procedure
            template.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplate | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
