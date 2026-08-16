from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_technique_list import ContentTechniqueList
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    tactic: str | Unset = UNSET,
    is_subtechnique: bool | Unset = UNSET,
    limit: int | Unset = 500,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["version"] = version

    params["q"] = q

    params["tactic"] = tactic

    params["isSubtechnique"] = is_subtechnique

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/techniques",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentTechniqueList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentTechniqueList.from_dict(response.json())

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
) -> Response[ContentTechniqueList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    tactic: str | Unset = UNSET,
    is_subtechnique: bool | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentTechniqueList | Problem]:
    """List ATT&CK techniques.

     Any authenticated subject (`content.read`). Returns techniques from
    **enabled** sources only. Filter by ATT&CK `version`, substring `q`
    (external id / name / description, case-insensitive), parent `tactic`
    external id, and `isSubtechnique`.

    Args:
        version (str | Unset):
        q (str | Unset):
        tactic (str | Unset):
        is_subtechnique (bool | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTechniqueList | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        q=q,
        tactic=tactic,
        is_subtechnique=is_subtechnique,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    tactic: str | Unset = UNSET,
    is_subtechnique: bool | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentTechniqueList | Problem | None:
    """List ATT&CK techniques.

     Any authenticated subject (`content.read`). Returns techniques from
    **enabled** sources only. Filter by ATT&CK `version`, substring `q`
    (external id / name / description, case-insensitive), parent `tactic`
    external id, and `isSubtechnique`.

    Args:
        version (str | Unset):
        q (str | Unset):
        tactic (str | Unset):
        is_subtechnique (bool | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTechniqueList | Problem
    """

    return sync_detailed(
        client=client,
        version=version,
        q=q,
        tactic=tactic,
        is_subtechnique=is_subtechnique,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    tactic: str | Unset = UNSET,
    is_subtechnique: bool | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentTechniqueList | Problem]:
    """List ATT&CK techniques.

     Any authenticated subject (`content.read`). Returns techniques from
    **enabled** sources only. Filter by ATT&CK `version`, substring `q`
    (external id / name / description, case-insensitive), parent `tactic`
    external id, and `isSubtechnique`.

    Args:
        version (str | Unset):
        q (str | Unset):
        tactic (str | Unset):
        is_subtechnique (bool | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentTechniqueList | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        q=q,
        tactic=tactic,
        is_subtechnique=is_subtechnique,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    version: str | Unset = UNSET,
    q: str | Unset = UNSET,
    tactic: str | Unset = UNSET,
    is_subtechnique: bool | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentTechniqueList | Problem | None:
    """List ATT&CK techniques.

     Any authenticated subject (`content.read`). Returns techniques from
    **enabled** sources only. Filter by ATT&CK `version`, substring `q`
    (external id / name / description, case-insensitive), parent `tactic`
    external id, and `isSubtechnique`.

    Args:
        version (str | Unset):
        q (str | Unset):
        tactic (str | Unset):
        is_subtechnique (bool | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentTechniqueList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            version=version,
            q=q,
            tactic=tactic,
            is_subtechnique=is_subtechnique,
            limit=limit,
        )
    ).parsed
