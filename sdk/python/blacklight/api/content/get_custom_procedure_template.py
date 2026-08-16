from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_procedure_template import ContentProcedureTemplate
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    template_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/custom/procedure-templates/{template_id}".format(
            template_id=quote(str(template_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentProcedureTemplate | Problem | None:
    if response.status_code == 200:
        response_200 = ContentProcedureTemplate.from_dict(response.json())

        return response_200

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
) -> Response[ContentProcedureTemplate | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    template_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentProcedureTemplate | Problem]:
    """Read one custom procedure template.

     Any authenticated subject. Answers `404` when the id is missing or not
    under the custom source.

    Args:
        template_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplate | Problem]
    """

    kwargs = _get_kwargs(
        template_id=template_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    template_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentProcedureTemplate | Problem | None:
    """Read one custom procedure template.

     Any authenticated subject. Answers `404` when the id is missing or not
    under the custom source.

    Args:
        template_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplate | Problem
    """

    return sync_detailed(
        template_id=template_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    template_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentProcedureTemplate | Problem]:
    """Read one custom procedure template.

     Any authenticated subject. Answers `404` when the id is missing or not
    under the custom source.

    Args:
        template_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplate | Problem]
    """

    kwargs = _get_kwargs(
        template_id=template_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    template_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentProcedureTemplate | Problem | None:
    """Read one custom procedure template.

     Any authenticated subject. Answers `404` when the id is missing or not
    under the custom source.

    Args:
        template_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplate | Problem
    """

    return (
        await asyncio_detailed(
            template_id=template_id,
            client=client,
        )
    ).parsed
