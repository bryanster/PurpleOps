from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.login_result import LoginResult
from ...models.problem import Problem
from ...models.recovery_code_request import RecoveryCodeRequest
from typing import cast


def _get_kwargs(
    *,
    body: RecoveryCodeRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/mfa/recovery/verify",
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
    body: RecoveryCodeRequest,
) -> Response[LoginResult | Problem]:
    """Complete a sign-in with a recovery code instead of an authenticator.

     The other way out of `mfa_required`, for the person whose phone is in a
    river (M1-007). It takes the same short-lived `bl_mfa` cookie as
    `POST /auth/mfa/totp/verify` and one of the codes issued when the
    authenticator was enrolled.

    A code works exactly once. On success it is marked used, the pending
    state is spent, and a session is issued with MFA **satisfied** — this is
    a full sign-in and not a diminished one, because someone holding a
    printed code has presented a second factor. `GET /auth/me` then reports
    `mfa.recoveryCodesRemaining` one lower; below three, warn them.

    Every way of failing is the same `401` as the TOTP endpoint's: a code
    that is not theirs, one already used, one that is not a code at all, an
    expired pending state and no pending state at all. Attempts are
    throttled per account and per client address alongside password and
    TOTP attempts (M1-004).

    Args:
        body (RecoveryCodeRequest): Body of `POST /auth/mfa/recovery/verify`: one recovery code,
            as the
            person has it written down.

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
    body: RecoveryCodeRequest,
) -> LoginResult | Problem | None:
    """Complete a sign-in with a recovery code instead of an authenticator.

     The other way out of `mfa_required`, for the person whose phone is in a
    river (M1-007). It takes the same short-lived `bl_mfa` cookie as
    `POST /auth/mfa/totp/verify` and one of the codes issued when the
    authenticator was enrolled.

    A code works exactly once. On success it is marked used, the pending
    state is spent, and a session is issued with MFA **satisfied** — this is
    a full sign-in and not a diminished one, because someone holding a
    printed code has presented a second factor. `GET /auth/me` then reports
    `mfa.recoveryCodesRemaining` one lower; below three, warn them.

    Every way of failing is the same `401` as the TOTP endpoint's: a code
    that is not theirs, one already used, one that is not a code at all, an
    expired pending state and no pending state at all. Attempts are
    throttled per account and per client address alongside password and
    TOTP attempts (M1-004).

    Args:
        body (RecoveryCodeRequest): Body of `POST /auth/mfa/recovery/verify`: one recovery code,
            as the
            person has it written down.

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
    body: RecoveryCodeRequest,
) -> Response[LoginResult | Problem]:
    """Complete a sign-in with a recovery code instead of an authenticator.

     The other way out of `mfa_required`, for the person whose phone is in a
    river (M1-007). It takes the same short-lived `bl_mfa` cookie as
    `POST /auth/mfa/totp/verify` and one of the codes issued when the
    authenticator was enrolled.

    A code works exactly once. On success it is marked used, the pending
    state is spent, and a session is issued with MFA **satisfied** — this is
    a full sign-in and not a diminished one, because someone holding a
    printed code has presented a second factor. `GET /auth/me` then reports
    `mfa.recoveryCodesRemaining` one lower; below three, warn them.

    Every way of failing is the same `401` as the TOTP endpoint's: a code
    that is not theirs, one already used, one that is not a code at all, an
    expired pending state and no pending state at all. Attempts are
    throttled per account and per client address alongside password and
    TOTP attempts (M1-004).

    Args:
        body (RecoveryCodeRequest): Body of `POST /auth/mfa/recovery/verify`: one recovery code,
            as the
            person has it written down.

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
    body: RecoveryCodeRequest,
) -> LoginResult | Problem | None:
    """Complete a sign-in with a recovery code instead of an authenticator.

     The other way out of `mfa_required`, for the person whose phone is in a
    river (M1-007). It takes the same short-lived `bl_mfa` cookie as
    `POST /auth/mfa/totp/verify` and one of the codes issued when the
    authenticator was enrolled.

    A code works exactly once. On success it is marked used, the pending
    state is spent, and a session is issued with MFA **satisfied** — this is
    a full sign-in and not a diminished one, because someone holding a
    printed code has presented a second factor. `GET /auth/me` then reports
    `mfa.recoveryCodesRemaining` one lower; below three, warn them.

    Every way of failing is the same `401` as the TOTP endpoint's: a code
    that is not theirs, one already used, one that is not a code at all, an
    expired pending state and no pending state at all. Attempts are
    throttled per account and per client address alongside password and
    TOTP attempts (M1-004).

    Args:
        body (RecoveryCodeRequest): Body of `POST /auth/mfa/recovery/verify`: one recovery code,
            as the
            person has it written down.

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
