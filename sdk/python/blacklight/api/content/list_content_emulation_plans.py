from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_emulation_plan_list import ContentEmulationPlanList
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    *,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 500,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["q"] = q

    params["technique"] = technique

    json_source_id: str | Unset = UNSET
    if not isinstance(source_id, Unset):
        json_source_id = str(source_id)
    params["sourceId"] = json_source_id

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/content/emulation-plans",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentEmulationPlanList | Problem | None:
    if response.status_code == 200:
        response_200 = ContentEmulationPlanList.from_dict(response.json())

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
) -> Response[ContentEmulationPlanList | Problem]:
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
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentEmulationPlanList | Problem]:
    """List CTID emulation plans.

     Any authenticated subject (`content.read`). Returns emulation plans
    from **enabled** sources only. Filter by substring `q` (external id /
    name / description / adversary name), ATT&CK `technique` external id
    (plans that have at least one step with that id), and optional
    `sourceId`.

    CTID is a rolling-head source: rows use version `current`. Catalog only
    in M2 — scenario import is M3-012.

    Args:
        q (str | Unset):
        technique (str | Unset):
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentEmulationPlanList | Problem]
    """

    kwargs = _get_kwargs(
        q=q,
        technique=technique,
        source_id=source_id,
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
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentEmulationPlanList | Problem | None:
    """List CTID emulation plans.

     Any authenticated subject (`content.read`). Returns emulation plans
    from **enabled** sources only. Filter by substring `q` (external id /
    name / description / adversary name), ATT&CK `technique` external id
    (plans that have at least one step with that id), and optional
    `sourceId`.

    CTID is a rolling-head source: rows use version `current`. Catalog only
    in M2 — scenario import is M3-012.

    Args:
        q (str | Unset):
        technique (str | Unset):
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentEmulationPlanList | Problem
    """

    return sync_detailed(
        client=client,
        q=q,
        technique=technique,
        source_id=source_id,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 500,
) -> Response[ContentEmulationPlanList | Problem]:
    """List CTID emulation plans.

     Any authenticated subject (`content.read`). Returns emulation plans
    from **enabled** sources only. Filter by substring `q` (external id /
    name / description / adversary name), ATT&CK `technique` external id
    (plans that have at least one step with that id), and optional
    `sourceId`.

    CTID is a rolling-head source: rows use version `current`. Catalog only
    in M2 — scenario import is M3-012.

    Args:
        q (str | Unset):
        technique (str | Unset):
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentEmulationPlanList | Problem]
    """

    kwargs = _get_kwargs(
        q=q,
        technique=technique,
        source_id=source_id,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    q: str | Unset = UNSET,
    technique: str | Unset = UNSET,
    source_id: UUID | Unset = UNSET,
    limit: int | Unset = 500,
) -> ContentEmulationPlanList | Problem | None:
    """List CTID emulation plans.

     Any authenticated subject (`content.read`). Returns emulation plans
    from **enabled** sources only. Filter by substring `q` (external id /
    name / description / adversary name), ATT&CK `technique` external id
    (plans that have at least one step with that id), and optional
    `sourceId`.

    CTID is a rolling-head source: rows use version `current`. Catalog only
    in M2 — scenario import is M3-012.

    Args:
        q (str | Unset):
        technique (str | Unset):
        source_id (UUID | Unset):
        limit (int | Unset):  Default: 500.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentEmulationPlanList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            q=q,
            technique=technique,
            source_id=source_id,
            limit=limit,
        )
    ).parsed
