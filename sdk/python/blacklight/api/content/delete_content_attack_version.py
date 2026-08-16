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


def _get_kwargs(
    version: str,
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/content/attack/versions/{version}".format(
            version=quote(str(version), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

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
    version: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Delete one ATT&CK version catalog.

     Administrators only (`content.manage`). Removes that version's object
    rows and version snapshot only — other installed versions are
    untouched.

    When anything outside content still references the pin, answers `409`
    with counts rather than cascading. In M2 those refs do not exist yet
    (M3 implements `References.AttackVersion`); a version with zero refs
    always deletes.

    Records activity `content.version.deleted`.

    Args:
        version (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    version: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Delete one ATT&CK version catalog.

     Administrators only (`content.manage`). Removes that version's object
    rows and version snapshot only — other installed versions are
    untouched.

    When anything outside content still references the pin, answers `409`
    with counts rather than cascading. In M2 those refs do not exist yet
    (M3 implements `References.AttackVersion`); a version with zero refs
    always deletes.

    Records activity `content.version.deleted`.

    Args:
        version (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        version=version,
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    version: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Delete one ATT&CK version catalog.

     Administrators only (`content.manage`). Removes that version's object
    rows and version snapshot only — other installed versions are
    untouched.

    When anything outside content still references the pin, answers `409`
    with counts rather than cascading. In M2 those refs do not exist yet
    (M3 implements `References.AttackVersion`); a version with zero refs
    always deletes.

    Records activity `content.version.deleted`.

    Args:
        version (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        version=version,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    version: str,
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Delete one ATT&CK version catalog.

     Administrators only (`content.manage`). Removes that version's object
    rows and version snapshot only — other installed versions are
    untouched.

    When anything outside content still references the pin, answers `409`
    with counts rather than cascading. In M2 those refs do not exist yet
    (M3 implements `References.AttackVersion`); a version with zero refs
    always deletes.

    Records activity `content.version.deleted`.

    Args:
        version (str):
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            version=version,
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
