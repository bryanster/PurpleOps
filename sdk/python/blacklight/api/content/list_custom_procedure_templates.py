from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_procedure_template_list import ContentProcedureTemplateList
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    platform: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["q"] = q

    params["technique"] = technique

    params["platform"] = platform

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/custom/procedure-templates",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentProcedureTemplateList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentProcedureTemplateList.from_dict(response.json())

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
) -> Response[ContentProcedureTemplateList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    platform: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentProcedureTemplateList | Problem]:
    """List custom procedure templates.

     Any authenticated subject (`content.read`). Returns procedure templates
    under the singleton `custom` source only. Structure is preserved
    (platforms, executor, command, cleanup, input args). Does not require
    an ATT&CK install.

    Args:
        q (str | Unset):
        technique (str | Unset):
        platform (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplateList | Problem]
    """

    kwargs = _get_kwargs(
        q=q,
        technique=technique,
        platform=platform,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    platform: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentProcedureTemplateList | Problem | None:
    """List custom procedure templates.

     Any authenticated subject (`content.read`). Returns procedure templates
    under the singleton `custom` source only. Structure is preserved
    (platforms, executor, command, cleanup, input args). Does not require
    an ATT&CK install.

    Args:
        q (str | Unset):
        technique (str | Unset):
        platform (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplateList | Problem
    """

    return sync_detailed(
        client=client,
        q=q,
        technique=technique,
        platform=platform,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    platform: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentProcedureTemplateList | Problem]:
    """List custom procedure templates.

     Any authenticated subject (`content.read`). Returns procedure templates
    under the singleton `custom` source only. Structure is preserved
    (platforms, executor, command, cleanup, input args). Does not require
    an ATT&CK install.

    Args:
        q (str | Unset):
        technique (str | Unset):
        platform (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentProcedureTemplateList | Problem]
    """

    kwargs = _get_kwargs(
        q=q,
        technique=technique,
        platform=platform,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    platform: str | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentProcedureTemplateList | Problem | None:
    """List custom procedure templates.

     Any authenticated subject (`content.read`). Returns procedure templates
    under the singleton `custom` source only. Structure is preserved
    (platforms, executor, command, cleanup, input args). Does not require
    an ATT&CK install.

    Args:
        q (str | Unset):
        technique (str | Unset):
        platform (str | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentProcedureTemplateList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            q=q,
            technique=technique,
            platform=platform,
            limit=limit,
        )
    ).parsed
