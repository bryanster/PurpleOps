from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.setup_state import SetupState
from typing import cast


def _get_kwargs() -> dict[str, Any]:
    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/setup",
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | SetupState | None:
    if response.status_code == 200:
        response_200 = SetupState.from_dict(response.json())

        return response_200

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
) -> Response[Problem | SetupState]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | SetupState]:
    """Read whether this installation has been through first-run setup.

     Administrators only (`settings.read`), because an administrator is the
    only caller who can act on the answer: the first-run wizard installs
    reference content, and installing content is `content.sync`.

    A fresh installation boots with an empty library — nothing is fetched at
    first boot — and answers `completed: false` until somebody finishes the
    wizard. `completed` records a *decision*, not an outcome: finishing it
    without installing anything is the right answer for an air-gapped
    deployment that will import an offline bundle later, and that
    installation must not be handed a screen it can never dismiss. What is
    actually installed is `GET /content/attack/versions`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SetupState]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | SetupState | None:
    """Read whether this installation has been through first-run setup.

     Administrators only (`settings.read`), because an administrator is the
    only caller who can act on the answer: the first-run wizard installs
    reference content, and installing content is `content.sync`.

    A fresh installation boots with an empty library — nothing is fetched at
    first boot — and answers `completed: false` until somebody finishes the
    wizard. `completed` records a *decision*, not an outcome: finishing it
    without installing anything is the right answer for an air-gapped
    deployment that will import an offline bundle later, and that
    installation must not be handed a screen it can never dismiss. What is
    actually installed is `GET /content/attack/versions`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SetupState
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | SetupState]:
    """Read whether this installation has been through first-run setup.

     Administrators only (`settings.read`), because an administrator is the
    only caller who can act on the answer: the first-run wizard installs
    reference content, and installing content is `content.sync`.

    A fresh installation boots with an empty library — nothing is fetched at
    first boot — and answers `completed: false` until somebody finishes the
    wizard. `completed` records a *decision*, not an outcome: finishing it
    without installing anything is the right answer for an air-gapped
    deployment that will import an offline bundle later, and that
    installation must not be handed a screen it can never dismiss. What is
    actually installed is `GET /content/attack/versions`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SetupState]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | SetupState | None:
    """Read whether this installation has been through first-run setup.

     Administrators only (`settings.read`), because an administrator is the
    only caller who can act on the answer: the first-run wizard installs
    reference content, and installing content is `content.sync`.

    A fresh installation boots with an empty library — nothing is fetched at
    first boot — and answers `completed: false` until somebody finishes the
    wizard. `completed` records a *decision*, not an outcome: finishing it
    without installing anything is the right answer for an air-gapped
    deployment that will import an offline bundle later, and that
    installation must not be handed a screen it can never dismiss. What is
    actually installed is `GET /content/attack/versions`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SetupState
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
