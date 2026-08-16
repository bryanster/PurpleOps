from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.setup_state import SetupState
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/setup/complete",
    }

    _kwargs["headers"] = headers
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
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | SetupState]:
    """Mark first-run setup as finished.

     Administrators only (`settings.manage`). Records that somebody reached
    the end of the wizard, so that the next sign-in lands on the product
    rather than back at it.

    Idempotent, and one-way from the API. A second call keeps the first
    call's timestamp and actor — when an installation was set up is not
    something a retried request should move — so a client that lost the
    response can simply send it again. `blctl setup reset` is the only way
    back to the wizard, and it is deliberately not an endpoint: putting an
    installation back into first-run state is an operator's decision made at
    the machine, not a button in a browser.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SetupState]
    """

    kwargs = _get_kwargs(
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | SetupState | None:
    """Mark first-run setup as finished.

     Administrators only (`settings.manage`). Records that somebody reached
    the end of the wizard, so that the next sign-in lands on the product
    rather than back at it.

    Idempotent, and one-way from the API. A second call keeps the first
    call's timestamp and actor — when an installation was set up is not
    something a retried request should move — so a client that lost the
    response can simply send it again. `blctl setup reset` is the only way
    back to the wizard, and it is deliberately not an endpoint: putting an
    installation back into first-run state is an operator's decision made at
    the machine, not a button in a browser.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SetupState
    """

    return sync_detailed(
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | SetupState]:
    """Mark first-run setup as finished.

     Administrators only (`settings.manage`). Records that somebody reached
    the end of the wizard, so that the next sign-in lands on the product
    rather than back at it.

    Idempotent, and one-way from the API. A second call keeps the first
    call's timestamp and actor — when an installation was set up is not
    something a retried request should move — so a client that lost the
    response can simply send it again. `blctl setup reset` is the only way
    back to the wizard, and it is deliberately not an endpoint: putting an
    installation back into first-run state is an operator's decision made at
    the machine, not a button in a browser.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SetupState]
    """

    kwargs = _get_kwargs(
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | SetupState | None:
    """Mark first-run setup as finished.

     Administrators only (`settings.manage`). Records that somebody reached
    the end of the wizard, so that the next sign-in lands on the product
    rather than back at it.

    Idempotent, and one-way from the API. A second call keeps the first
    call's timestamp and actor — when an installation was set up is not
    something a retried request should move — so a client that lost the
    response can simply send it again. `blctl setup reset` is the only way
    back to the wizard, and it is deliberately not an endpoint: putting an
    installation back into first-run state is an operator's decision made at
    the machine, not a button in a browser.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SetupState
    """

    return (
        await asyncio_detailed(
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
