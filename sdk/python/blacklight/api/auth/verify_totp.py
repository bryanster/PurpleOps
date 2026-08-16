from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.login_result import LoginResult
from ...models.problem import Problem
from ...models.totp_code_request import TOTPCodeRequest
from typing import cast


def _get_kwargs(
    *,
    body: TOTPCodeRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/mfa/totp/verify",
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
    client: AuthenticatedClient,
    body: TOTPCodeRequest,
) -> Response[LoginResult | Problem]:
    """Complete a sign-in by presenting a code from your authenticator.

     The second half of a sign-in that answered `mfa_required`. It carries no
    session — that is the point: the credential is the short-lived
    `bl_mfa` cookie set by `POST /auth/login`, which authorizes nothing
    except this endpoint and expires in minutes.

    On success the pending state is spent, a session is issued with MFA
    satisfied, and the `bl_mfa` cookie is cleared. One correct code buys
    exactly one session.

    Every way of failing is the same `401`: a wrong code, a code already
    used, an expired pending state, and no pending state at all. Attempts
    are throttled per account and per client address alongside password
    attempts (M1-004), and the `429` is identical whichever limit closed.

    Args:
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

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
    client: AuthenticatedClient,
    body: TOTPCodeRequest,
) -> LoginResult | Problem | None:
    """Complete a sign-in by presenting a code from your authenticator.

     The second half of a sign-in that answered `mfa_required`. It carries no
    session — that is the point: the credential is the short-lived
    `bl_mfa` cookie set by `POST /auth/login`, which authorizes nothing
    except this endpoint and expires in minutes.

    On success the pending state is spent, a session is issued with MFA
    satisfied, and the `bl_mfa` cookie is cleared. One correct code buys
    exactly one session.

    Every way of failing is the same `401`: a wrong code, a code already
    used, an expired pending state, and no pending state at all. Attempts
    are throttled per account and per client address alongside password
    attempts (M1-004), and the `429` is identical whichever limit closed.

    Args:
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

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
    client: AuthenticatedClient,
    body: TOTPCodeRequest,
) -> Response[LoginResult | Problem]:
    """Complete a sign-in by presenting a code from your authenticator.

     The second half of a sign-in that answered `mfa_required`. It carries no
    session — that is the point: the credential is the short-lived
    `bl_mfa` cookie set by `POST /auth/login`, which authorizes nothing
    except this endpoint and expires in minutes.

    On success the pending state is spent, a session is issued with MFA
    satisfied, and the `bl_mfa` cookie is cleared. One correct code buys
    exactly one session.

    Every way of failing is the same `401`: a wrong code, a code already
    used, an expired pending state, and no pending state at all. Attempts
    are throttled per account and per client address alongside password
    attempts (M1-004), and the `429` is identical whichever limit closed.

    Args:
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

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
    client: AuthenticatedClient,
    body: TOTPCodeRequest,
) -> LoginResult | Problem | None:
    """Complete a sign-in by presenting a code from your authenticator.

     The second half of a sign-in that answered `mfa_required`. It carries no
    session — that is the point: the credential is the short-lived
    `bl_mfa` cookie set by `POST /auth/login`, which authorizes nothing
    except this endpoint and expires in minutes.

    On success the pending state is spent, a session is issued with MFA
    satisfied, and the `bl_mfa` cookie is cleared. One correct code buys
    exactly one session.

    Every way of failing is the same `401`: a wrong code, a code already
    used, an expired pending state, and no pending state at all. Attempts
    are throttled per account and per client address alongside password
    attempts (M1-004), and the `429` is identical whichever limit closed.

    Args:
        body (TOTPCodeRequest): A six-digit code from an authenticator app. The same body confirms
            an
            enrolment and completes a sign-in.

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
