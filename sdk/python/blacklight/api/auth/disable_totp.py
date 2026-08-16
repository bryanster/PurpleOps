from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.disable_totp_request import DisableTOTPRequest
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: DisableTOTPRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/auth/mfa/totp",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

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
    *,
    client: AuthenticatedClient | Client,
    body: DisableTOTPRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Remove your authenticator.

     Requires the current password, for the same reason changing a password
    does: a session left open on a shared machine must not be enough to take
    the second factor off an account.

    The recovery codes go with it (M1-007). They stand in for the
    authenticator, so leaving them behind would mean a second factor that
    was removed is still presentable — and the next enrolment mints its own
    set, so keeping these could only ever mean two live sets.

    Refused with `403` when a second factor is required of this person
    (M1-008) — by the platform policy or by their own `mfaEnforced` flag,
    which are the same answer here. Removing it would leave an account
    subject to a requirement it can no longer satisfy, which is a lockout
    rather than a choice. `mfa.required` on `GET /auth/me` says in advance
    whether this call will be refused.

    Args:
        x_csrf_token (str | Unset):
        body (DisableTOTPRequest): Body of `DELETE /auth/mfa/totp`. The current password is
            required for the
            same reason `ChangePasswordRequest` asks for it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: DisableTOTPRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Remove your authenticator.

     Requires the current password, for the same reason changing a password
    does: a session left open on a shared machine must not be enough to take
    the second factor off an account.

    The recovery codes go with it (M1-007). They stand in for the
    authenticator, so leaving them behind would mean a second factor that
    was removed is still presentable — and the next enrolment mints its own
    set, so keeping these could only ever mean two live sets.

    Refused with `403` when a second factor is required of this person
    (M1-008) — by the platform policy or by their own `mfaEnforced` flag,
    which are the same answer here. Removing it would leave an account
    subject to a requirement it can no longer satisfy, which is a lockout
    rather than a choice. `mfa.required` on `GET /auth/me` says in advance
    whether this call will be refused.

    Args:
        x_csrf_token (str | Unset):
        body (DisableTOTPRequest): Body of `DELETE /auth/mfa/totp`. The current password is
            required for the
            same reason `ChangePasswordRequest` asks for it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: DisableTOTPRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Remove your authenticator.

     Requires the current password, for the same reason changing a password
    does: a session left open on a shared machine must not be enough to take
    the second factor off an account.

    The recovery codes go with it (M1-007). They stand in for the
    authenticator, so leaving them behind would mean a second factor that
    was removed is still presentable — and the next enrolment mints its own
    set, so keeping these could only ever mean two live sets.

    Refused with `403` when a second factor is required of this person
    (M1-008) — by the platform policy or by their own `mfaEnforced` flag,
    which are the same answer here. Removing it would leave an account
    subject to a requirement it can no longer satisfy, which is a lockout
    rather than a choice. `mfa.required` on `GET /auth/me` says in advance
    whether this call will be refused.

    Args:
        x_csrf_token (str | Unset):
        body (DisableTOTPRequest): Body of `DELETE /auth/mfa/totp`. The current password is
            required for the
            same reason `ChangePasswordRequest` asks for it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: DisableTOTPRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Any | Problem | None:
    """Remove your authenticator.

     Requires the current password, for the same reason changing a password
    does: a session left open on a shared machine must not be enough to take
    the second factor off an account.

    The recovery codes go with it (M1-007). They stand in for the
    authenticator, so leaving them behind would mean a second factor that
    was removed is still presentable — and the next enrolment mints its own
    set, so keeping these could only ever mean two live sets.

    Refused with `403` when a second factor is required of this person
    (M1-008) — by the platform policy or by their own `mfaEnforced` flag,
    which are the same answer here. Removing it would leave an account
    subject to a requirement it can no longer satisfy, which is a lockout
    rather than a choice. `mfa.required` on `GET /auth/me` says in advance
    whether this call will be refused.

    Args:
        x_csrf_token (str | Unset):
        body (DisableTOTPRequest): Body of `DELETE /auth/mfa/totp`. The current password is
            required for the
            same reason `ChangePasswordRequest` asks for it.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
