from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.recovery_codes import RecoveryCodes
from ...models.regenerate_recovery_codes_request import RegenerateRecoveryCodesRequest
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: RegenerateRecoveryCodesRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/mfa/recovery/regenerate",
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
    body: RegenerateRecoveryCodesRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RecoveryCodes]:
    """Replace your recovery codes with a fresh set.

     Mints ten new codes and **invalidates every previous one**, including
    the codes that were never used. Somebody regenerating because a printout
    went missing is telling us the missing printout must stop working.

    It asks for two things. The current password, so that a session left
    open on a shared machine cannot mint credentials that walk past a second
    factor. And a session that has already satisfied MFA — `403` otherwise.
    Deliberately *not* a fresh code from the authenticator: signing in with
    a recovery code produces a satisfied session, so this is reachable by
    the person whose phone is gone, which is the case recovery codes exist
    for.

    `409` when no authenticator is enrolled: codes stand in for one, and
    there has to be one to stand in for.

    Like the enrolment response, this is the only time the new codes exist
    outside the client.

    Args:
        x_csrf_token (str | Unset):
        body (RegenerateRecoveryCodesRequest): Body of `POST /auth/mfa/recovery/regenerate`. The
            current password is
            required for the same reason `ChangePasswordRequest` asks for it; the
            second factor is not in the body, it is the requirement that this
            session has already satisfied one.

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
    body: RegenerateRecoveryCodesRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RecoveryCodes | None:
    """Replace your recovery codes with a fresh set.

     Mints ten new codes and **invalidates every previous one**, including
    the codes that were never used. Somebody regenerating because a printout
    went missing is telling us the missing printout must stop working.

    It asks for two things. The current password, so that a session left
    open on a shared machine cannot mint credentials that walk past a second
    factor. And a session that has already satisfied MFA — `403` otherwise.
    Deliberately *not* a fresh code from the authenticator: signing in with
    a recovery code produces a satisfied session, so this is reachable by
    the person whose phone is gone, which is the case recovery codes exist
    for.

    `409` when no authenticator is enrolled: codes stand in for one, and
    there has to be one to stand in for.

    Like the enrolment response, this is the only time the new codes exist
    outside the client.

    Args:
        x_csrf_token (str | Unset):
        body (RegenerateRecoveryCodesRequest): Body of `POST /auth/mfa/recovery/regenerate`. The
            current password is
            required for the same reason `ChangePasswordRequest` asks for it; the
            second factor is not in the body, it is the requirement that this
            session has already satisfied one.

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
    body: RegenerateRecoveryCodesRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | RecoveryCodes]:
    """Replace your recovery codes with a fresh set.

     Mints ten new codes and **invalidates every previous one**, including
    the codes that were never used. Somebody regenerating because a printout
    went missing is telling us the missing printout must stop working.

    It asks for two things. The current password, so that a session left
    open on a shared machine cannot mint credentials that walk past a second
    factor. And a session that has already satisfied MFA — `403` otherwise.
    Deliberately *not* a fresh code from the authenticator: signing in with
    a recovery code produces a satisfied session, so this is reachable by
    the person whose phone is gone, which is the case recovery codes exist
    for.

    `409` when no authenticator is enrolled: codes stand in for one, and
    there has to be one to stand in for.

    Like the enrolment response, this is the only time the new codes exist
    outside the client.

    Args:
        x_csrf_token (str | Unset):
        body (RegenerateRecoveryCodesRequest): Body of `POST /auth/mfa/recovery/regenerate`. The
            current password is
            required for the same reason `ChangePasswordRequest` asks for it; the
            second factor is not in the body, it is the requirement that this
            session has already satisfied one.

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
    body: RegenerateRecoveryCodesRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Problem | RecoveryCodes | None:
    """Replace your recovery codes with a fresh set.

     Mints ten new codes and **invalidates every previous one**, including
    the codes that were never used. Somebody regenerating because a printout
    went missing is telling us the missing printout must stop working.

    It asks for two things. The current password, so that a session left
    open on a shared machine cannot mint credentials that walk past a second
    factor. And a session that has already satisfied MFA — `403` otherwise.
    Deliberately *not* a fresh code from the authenticator: signing in with
    a recovery code produces a satisfied session, so this is reachable by
    the person whose phone is gone, which is the case recovery codes exist
    for.

    `409` when no authenticator is enrolled: codes stand in for one, and
    there has to be one to stand in for.

    Like the enrolment response, this is the only time the new codes exist
    outside the client.

    Args:
        x_csrf_token (str | Unset):
        body (RegenerateRecoveryCodesRequest): Body of `POST /auth/mfa/recovery/regenerate`. The
            current password is
            required for the same reason `ChangePasswordRequest` asks for it; the
            second factor is not in the body, it is the requirement that this
            session has already satisfied one.

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
