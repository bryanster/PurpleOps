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
    source_id: UUID,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/content/sources/{source_id}".format(
            source_id=quote(str(source_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Hard-delete a content source and its content subtree.

     Administrators only (`content.manage`). Removes the source and every
    version, object, and job row that names it, in one write transaction.
    There is no path from content into app, so engagement history is never
    touched.

    The **custom** seed source cannot be deleted (`409`): user-authored rows
    need a home. Builtin upstream seeds may be deleted; disabling is the
    normal path, and re-seed is not automatic.

    When a later milestone adds engagement-side references, a source that
    is still referenced will answer `409` with counts rather than cascade
    into war-room data. In M2 those refs do not exist, so a non-custom
    source always deletes.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Hard-delete a content source and its content subtree.

     Administrators only (`content.manage`). Removes the source and every
    version, object, and job row that names it, in one write transaction.
    There is no path from content into app, so engagement history is never
    touched.

    The **custom** seed source cannot be deleted (`409`): user-authored rows
    need a home. Builtin upstream seeds may be deleted; disabling is the
    normal path, and re-seed is not automatic.

    When a later milestone adds engagement-side references, a source that
    is still referenced will answer `409` with counts rather than cascade
    into war-room data. In M2 those refs do not exist, so a non-custom
    source always deletes.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        source_id=source_id,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Hard-delete a content source and its content subtree.

     Administrators only (`content.manage`). Removes the source and every
    version, object, and job row that names it, in one write transaction.
    There is no path from content into app, so engagement history is never
    touched.

    The **custom** seed source cannot be deleted (`409`): user-authored rows
    need a home. Builtin upstream seeds may be deleted; disabling is the
    normal path, and re-seed is not automatic.

    When a later milestone adds engagement-side references, a source that
    is still referenced will answer `409` with counts rather than cascade
    into war-room data. In M2 those refs do not exist, so a non-custom
    source always deletes.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        source_id=source_id,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    source_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Hard-delete a content source and its content subtree.

     Administrators only (`content.manage`). Removes the source and every
    version, object, and job row that names it, in one write transaction.
    There is no path from content into app, so engagement history is never
    touched.

    The **custom** seed source cannot be deleted (`409`): user-authored rows
    need a home. Builtin upstream seeds may be deleted; disabling is the
    normal path, and re-seed is not automatic.

    When a later milestone adds engagement-side references, a source that
    is still referenced will answer `409` with counts rather than cascade
    into war-room data. In M2 those refs do not exist, so a non-custom
    source always deletes.

    Args:
        source_id (UUID):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            source_id=source_id,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
