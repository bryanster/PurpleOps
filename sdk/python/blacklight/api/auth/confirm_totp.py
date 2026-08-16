from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.recovery_codes import RecoveryCodes
from ...models.totp_code_request import TOTPCodeRequest
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: TOTPCodeRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/mfa/totp/confirm",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RecoveryCodes | None:
    if response.status_code == 200:
        response_200 = RecoveryCodes.from_dict(response.json())

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
) -> Response[Problem | RecoveryCodes]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: TOTPCodeRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RecoveryCodes]:
    """Confirm an enrolment by presenting a code from it.

     Proves the authenticator was actually set up. On success the enrolment
    becomes the second factor this account is asked for at sign-in, this
    session is marked as having satisfied MFA, and it is rotated onto a new
    token — satisfying a factor is a privilege change, and PLAN.md §4 wants
    the token to change whenever that is true.

    It also mints ten **recovery codes** (M1-007) and returns them in the
    body. This is the only response in the API that ever carries them: there
    is no endpoint that reads them back, because the server keeps only their
    hashes. A client that does not put them in front of the person right
    now has lost them, and the only remedy is
    `POST /auth/mfa/recovery/regenerate`, which mints a different set.

    A wrong code is a `400` naming the `code` field, not a `401`: the caller
    is signed in and this is a form to correct, not a session to re-establish.
    It is deliberately not throttled — the only secret being guessed is the
    caller's own, and getting it right grants them nothing they did not
    already have.

    Args:
        x_csrf_token (str | Unset):
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RecoveryCodes]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: TOTPCodeRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RecoveryCodes | None:
    """Confirm an enrolment by presenting a code from it.

     Proves the authenticator was actually set up. On success the enrolment
    becomes the second factor this account is asked for at sign-in, this
    session is marked as having satisfied MFA, and it is rotated onto a new
    token — satisfying a factor is a privilege change, and PLAN.md §4 wants
    the token to change whenever that is true.

    It also mints ten **recovery codes** (M1-007) and returns them in the
    body. This is the only response in the API that ever carries them: there
    is no endpoint that reads them back, because the server keeps only their
    hashes. A client that does not put them in front of the person right
    now has lost them, and the only remedy is
    `POST /auth/mfa/recovery/regenerate`, which mints a different set.

    A wrong code is a `400` naming the `code` field, not a `401`: the caller
    is signed in and this is a form to correct, not a session to re-establish.
    It is deliberately not throttled — the only secret being guessed is the
    caller's own, and getting it right grants them nothing they did not
    already have.

    Args:
        x_csrf_token (str | Unset):
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RecoveryCodes
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: TOTPCodeRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RecoveryCodes]:
    """Confirm an enrolment by presenting a code from it.

     Proves the authenticator was actually set up. On success the enrolment
    becomes the second factor this account is asked for at sign-in, this
    session is marked as having satisfied MFA, and it is rotated onto a new
    token — satisfying a factor is a privilege change, and PLAN.md §4 wants
    the token to change whenever that is true.

    It also mints ten **recovery codes** (M1-007) and returns them in the
    body. This is the only response in the API that ever carries them: there
    is no endpoint that reads them back, because the server keeps only their
    hashes. A client that does not put them in front of the person right
    now has lost them, and the only remedy is
    `POST /auth/mfa/recovery/regenerate`, which mints a different set.

    A wrong code is a `400` naming the `code` field, not a `401`: the caller
    is signed in and this is a form to correct, not a session to re-establish.
    It is deliberately not throttled — the only secret being guessed is the
    caller's own, and getting it right grants them nothing they did not
    already have.

    Args:
        x_csrf_token (str | Unset):
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RecoveryCodes]
    """

    kwargs = _get_kwargs(
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: TOTPCodeRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RecoveryCodes | None:
    """Confirm an enrolment by presenting a code from it.

     Proves the authenticator was actually set up. On success the enrolment
    becomes the second factor this account is asked for at sign-in, this
    session is marked as having satisfied MFA, and it is rotated onto a new
    token — satisfying a factor is a privilege change, and PLAN.md §4 wants
    the token to change whenever that is true.

    It also mints ten **recovery codes** (M1-007) and returns them in the
    body. This is the only response in the API that ever carries them: there
    is no endpoint that reads them back, because the server keeps only their
    hashes. A client that does not put them in front of the person right
    now has lost them, and the only remedy is
    `POST /auth/mfa/recovery/regenerate`, which mints a different set.

    A wrong code is a `400` naming the `code` field, not a `401`: the caller
    is signed in and this is a form to correct, not a session to re-establish.
    It is deliberately not throttled — the only secret being guessed is the
    caller's own, and getting it right grants them nothing they did not
    already have.

    Args:
        x_csrf_token (str | Unset):
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RecoveryCodes
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
