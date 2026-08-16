from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_technique_detail import ContentTechniqueDetail
from ...models.problem import Problem
from typing import cast
from uuid import UUID


def _get_kwargs(
    technique_id: UUID,
) -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/techniques/{technique_id}".format(
            technique_id=quote(str(technique_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentTechniqueDetail | Problem | None:
    if response.status_code == 200:
        response_200 = ContentTechniqueDetail.from_dict(response.json())

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
) -> Response[ContentTechniqueDetail | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    technique_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentTechniqueDetail | Problem]:
    """Read one ATT&CK technique.

     Any authenticated subject. Includes tactic and mitigation external ids.
    Techniques from disabled sources answer `404`.

    Args:
        technique_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTechniqueDetail | Problem]
    """

    kwargs = _get_kwargs(
        technique_id=technique_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    technique_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentTechniqueDetail | Problem | None:
    """Read one ATT&CK technique.

     Any authenticated subject. Includes tactic and mitigation external ids.
    Techniques from disabled sources answer `404`.

    Args:
        technique_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTechniqueDetail | Problem
    """

    return sync_detailed(
        technique_id=technique_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    technique_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[ContentTechniqueDetail | Problem]:
    """Read one ATT&CK technique.

     Any authenticated subject. Includes tactic and mitigation external ids.
    Techniques from disabled sources answer `404`.

    Args:
        technique_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTechniqueDetail | Problem]
    """

    kwargs = _get_kwargs(
        technique_id=technique_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    technique_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> ContentTechniqueDetail | Problem | None:
    """Read one ATT&CK technique.

     Any authenticated subject. Includes tactic and mitigation external ids.
    Techniques from disabled sources answer `404`.

    Args:
        technique_id (UUID):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTechniqueDetail | Problem
    """

    return (
        await asyncio_detailed(
            technique_id=technique_id,
            client=client,
        )
    ).parsed
