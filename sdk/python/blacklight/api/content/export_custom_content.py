from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_custom_export import ContentCustomExport
from ...models.export_custom_content_format import ExportCustomContentFormat
from ...models.export_custom_content_type import ExportCustomContentType
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    type_: ExportCustomContentType | Unset = UNSET,
    format_: ExportCustomContentFormat | Unset = ExportCustomContentFormat.YAML,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    json_type_: str | Unset = UNSET
    if not isinstance(type_, Unset):
        json_type_ = type_.value

    params["type"] = json_type_

    json_format_: str | Unset = UNSET
    if not isinstance(format_, Unset):
        json_format_ = format_.value

    params["format"] = json_format_

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/custom/export",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentCustomExport | Problem | None:
    if response.status_code == 200:
        response_200 = ContentCustomExport.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContentCustomExport | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    type_: ExportCustomContentType | Unset = UNSET,
    format_: ExportCustomContentFormat | Unset = ExportCustomContentFormat.YAML,
) -> Response[ContentCustomExport | Problem]:
    """Export custom content as YAML or JSON.

     Any authenticated subject (`content.read`). Returns a document
    suitable for re-import (M2-012) containing procedure templates,
    detection rules, and/or notes under the `custom` source. Header
    comments (YAML) or a `meta` block (JSON) carry license/attribution
    for the installation's custom library.

    `type` narrows to one object family; omit it for all three.
    `format` selects serialization (`yaml` default, or `json`).

    Args:
        type_ (ExportCustomContentType | Unset):
        format_ (ExportCustomContentFormat | Unset):  Default: ExportCustomContentFormat.YAML.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentCustomExport | Problem]
    """

    kwargs = _get_kwargs(
        type_=type_,
        format_=format_,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    type_: ExportCustomContentType | Unset = UNSET,
    format_: ExportCustomContentFormat | Unset = ExportCustomContentFormat.YAML,
) -> ContentCustomExport | Problem | None:
    """Export custom content as YAML or JSON.

     Any authenticated subject (`content.read`). Returns a document
    suitable for re-import (M2-012) containing procedure templates,
    detection rules, and/or notes under the `custom` source. Header
    comments (YAML) or a `meta` block (JSON) carry license/attribution
    for the installation's custom library.

    `type` narrows to one object family; omit it for all three.
    `format` selects serialization (`yaml` default, or `json`).

    Args:
        type_ (ExportCustomContentType | Unset):
        format_ (ExportCustomContentFormat | Unset):  Default: ExportCustomContentFormat.YAML.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentCustomExport | Problem
    """

    return sync_detailed(
        client=client,
        type_=type_,
        format_=format_,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    type_: ExportCustomContentType | Unset = UNSET,
    format_: ExportCustomContentFormat | Unset = ExportCustomContentFormat.YAML,
) -> Response[ContentCustomExport | Problem]:
    """Export custom content as YAML or JSON.

     Any authenticated subject (`content.read`). Returns a document
    suitable for re-import (M2-012) containing procedure templates,
    detection rules, and/or notes under the `custom` source. Header
    comments (YAML) or a `meta` block (JSON) carry license/attribution
    for the installation's custom library.

    `type` narrows to one object family; omit it for all three.
    `format` selects serialization (`yaml` default, or `json`).

    Args:
        type_ (ExportCustomContentType | Unset):
        format_ (ExportCustomContentFormat | Unset):  Default: ExportCustomContentFormat.YAML.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentCustomExport | Problem]
    """

    kwargs = _get_kwargs(
        type_=type_,
        format_=format_,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    type_: ExportCustomContentType | Unset = UNSET,
    format_: ExportCustomContentFormat | Unset = ExportCustomContentFormat.YAML,
) -> ContentCustomExport | Problem | None:
    """Export custom content as YAML or JSON.

     Any authenticated subject (`content.read`). Returns a document
    suitable for re-import (M2-012) containing procedure templates,
    detection rules, and/or notes under the `custom` source. Header
    comments (YAML) or a `meta` block (JSON) carry license/attribution
    for the installation's custom library.

    `type` narrows to one object family; omit it for all three.
    `format` selects serialization (`yaml` default, or `json`).

    Args:
        type_ (ExportCustomContentType | Unset):
        format_ (ExportCustomContentFormat | Unset):  Default: ExportCustomContentFormat.YAML.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentCustomExport | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            type_=type_,
            format_=format_,
        )
    ).parsed
