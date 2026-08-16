from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.login_request import LoginRequest
from ...models.login_result import LoginResult
from ...models.problem import Problem
from typing import cast


def _get_kwargs(
    *,
    body: LoginRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/login",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> LoginResult | Problem | None:
    if response.status_code == 200:
        response_200 = LoginResult.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[LoginResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: LoginRequest,
) -> Response[LoginResult | Problem]:
    """Sign in with an email address and password.

     Public, because the caller has no session yet — that is what this
    endpoint issues.

    Every way of failing produces the *same* 401 with the same body: a wrong
    password, an address nobody holds, and a disabled account are
    indistinguishable, so that this endpoint cannot be used to find out who
    has an account here. The server spends the same work on each, too.

    A 200 does not always mean a session. When a second factor is required
    and the caller has one, `status` is `mfa_required`, no *session* cookie
    is set, and they must post a code to `POST /auth/mfa/totp/verify` — or a
    recovery code to `POST /auth/mfa/recovery/verify` (M1-007) — before
    anything is signed in (M1-006).

    When one is required and they have **none**, `status` is
    `mfa_enrolment_required`: a session cookie is set and it may do exactly
    one thing, which is enrol (M1-008). Read `status` rather than assuming.

    Attempts are throttled per account and per client address (M1-004). Once
    either limit trips, every attempt is a 429 with a `Retry-After` — the
    right password included, because a lockout that the right password ends
    is not a lockout. The 429 is identical for an account that exists and one
    that does not.

    Args:
        body (LoginRequest): Credentials for `POST /auth/login`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[LoginResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: LoginRequest,
) -> LoginResult | Problem | None:
    """Sign in with an email address and password.

     Public, because the caller has no session yet — that is what this
    endpoint issues.

    Every way of failing produces the *same* 401 with the same body: a wrong
    password, an address nobody holds, and a disabled account are
    indistinguishable, so that this endpoint cannot be used to find out who
    has an account here. The server spends the same work on each, too.

    A 200 does not always mean a session. When a second factor is required
    and the caller has one, `status` is `mfa_required`, no *session* cookie
    is set, and they must post a code to `POST /auth/mfa/totp/verify` — or a
    recovery code to `POST /auth/mfa/recovery/verify` (M1-007) — before
    anything is signed in (M1-006).

    When one is required and they have **none**, `status` is
    `mfa_enrolment_required`: a session cookie is set and it may do exactly
    one thing, which is enrol (M1-008). Read `status` rather than assuming.

    Attempts are throttled per account and per client address (M1-004). Once
    either limit trips, every attempt is a 429 with a `Retry-After` — the
    right password included, because a lockout that the right password ends
    is not a lockout. The 429 is identical for an account that exists and one
    that does not.

    Args:
        body (LoginRequest): Credentials for `POST /auth/login`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        LoginResult | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: LoginRequest,
) -> Response[LoginResult | Problem]:
    """Sign in with an email address and password.

     Public, because the caller has no session yet — that is what this
    endpoint issues.

    Every way of failing produces the *same* 401 with the same body: a wrong
    password, an address nobody holds, and a disabled account are
    indistinguishable, so that this endpoint cannot be used to find out who
    has an account here. The server spends the same work on each, too.

    A 200 does not always mean a session. When a second factor is required
    and the caller has one, `status` is `mfa_required`, no *session* cookie
    is set, and they must post a code to `POST /auth/mfa/totp/verify` — or a
    recovery code to `POST /auth/mfa/recovery/verify` (M1-007) — before
    anything is signed in (M1-006).

    When one is required and they have **none**, `status` is
    `mfa_enrolment_required`: a session cookie is set and it may do exactly
    one thing, which is enrol (M1-008). Read `status` rather than assuming.

    Attempts are throttled per account and per client address (M1-004). Once
    either limit trips, every attempt is a 429 with a `Retry-After` — the
    right password included, because a lockout that the right password ends
    is not a lockout. The 429 is identical for an account that exists and one
    that does not.

    Args:
        body (LoginRequest): Credentials for `POST /auth/login`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[LoginResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: LoginRequest,
) -> LoginResult | Problem | None:
    """Sign in with an email address and password.

     Public, because the caller has no session yet — that is what this
    endpoint issues.

    Every way of failing produces the *same* 401 with the same body: a wrong
    password, an address nobody holds, and a disabled account are
    indistinguishable, so that this endpoint cannot be used to find out who
    has an account here. The server spends the same work on each, too.

    A 200 does not always mean a session. When a second factor is required
    and the caller has one, `status` is `mfa_required`, no *session* cookie
    is set, and they must post a code to `POST /auth/mfa/totp/verify` — or a
    recovery code to `POST /auth/mfa/recovery/verify` (M1-007) — before
    anything is signed in (M1-006).

    When one is required and they have **none**, `status` is
    `mfa_enrolment_required`: a session cookie is set and it may do exactly
    one thing, which is enrol (M1-008). Read `status` rather than assuming.

    Attempts are throttled per account and per client address (M1-004). Once
    either limit trips, every attempt is a 429 with a `Retry-After` — the
    right password included, because a lockout that the right password ends
    is not a lockout. The 429 is identical for an account that exists and one
    that does not.

    Args:
        body (LoginRequest): Credentials for `POST /auth/login`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        LoginResult | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
