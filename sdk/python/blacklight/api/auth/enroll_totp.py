from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.problem import Problem
from ...models.totp_enrolment import TOTPEnrolment
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
        "url": "/auth/mfa/totp/enroll",
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TOTPEnrolment | None:
    if response.status_code == 200:
        response_200 = TOTPEnrolment.from_dict(response.json())

        return response_200

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
) -> Response[Problem | TOTPEnrolment]:
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
) -> Response[Problem | TOTPEnrolment]:
    """Start enrolling an authenticator app.

     Mints a fresh shared secret and returns it three ways: as an
    `otpauth://` URI, as a QR code to point a camera at, and as the base32
    string to type in when the camera will not focus.

    The secret is **unconfirmed** until `POST /auth/mfa/totp/confirm`
    succeeds. An unconfirmed secret gates nothing — a browser closed between
    this call and that one leaves the account exactly as it was, which is
    what stops a half-finished enrolment locking somebody out.

    Calling this again replaces an unconfirmed secret, so a re-scan works.
    It refuses with `409` when a *confirmed* authenticator already exists:
    replacing one is `DELETE /auth/mfa/totp` followed by this, and that
    needs the current password.

    This is the only response that ever carries the secret.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TOTPEnrolment]
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
) -> Problem | TOTPEnrolment | None:
    """Start enrolling an authenticator app.

     Mints a fresh shared secret and returns it three ways: as an
    `otpauth://` URI, as a QR code to point a camera at, and as the base32
    string to type in when the camera will not focus.

    The secret is **unconfirmed** until `POST /auth/mfa/totp/confirm`
    succeeds. An unconfirmed secret gates nothing — a browser closed between
    this call and that one leaves the account exactly as it was, which is
    what stops a half-finished enrolment locking somebody out.

    Calling this again replaces an unconfirmed secret, so a re-scan works.
    It refuses with `409` when a *confirmed* authenticator already exists:
    replacing one is `DELETE /auth/mfa/totp` followed by this, and that
    needs the current password.

    This is the only response that ever carries the secret.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TOTPEnrolment
    """

    return sync_detailed(
        client=client,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    x_csrf_token: str | Unset = UNSET,
) -> Response[Problem | TOTPEnrolment]:
    """Start enrolling an authenticator app.

     Mints a fresh shared secret and returns it three ways: as an
    `otpauth://` URI, as a QR code to point a camera at, and as the base32
    string to type in when the camera will not focus.

    The secret is **unconfirmed** until `POST /auth/mfa/totp/confirm`
    succeeds. An unconfirmed secret gates nothing — a browser closed between
    this call and that one leaves the account exactly as it was, which is
    what stops a half-finished enrolment locking somebody out.

    Calling this again replaces an unconfirmed secret, so a re-scan works.
    It refuses with `409` when a *confirmed* authenticator already exists:
    replacing one is `DELETE /auth/mfa/totp` followed by this, and that
    needs the current password.

    This is the only response that ever carries the secret.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TOTPEnrolment]
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
) -> Problem | TOTPEnrolment | None:
    """Start enrolling an authenticator app.

     Mints a fresh shared secret and returns it three ways: as an
    `otpauth://` URI, as a QR code to point a camera at, and as the base32
    string to type in when the camera will not focus.

    The secret is **unconfirmed** until `POST /auth/mfa/totp/confirm`
    succeeds. An unconfirmed secret gates nothing — a browser closed between
    this call and that one leaves the account exactly as it was, which is
    what stops a half-finished enrolment locking somebody out.

    Calling this again replaces an unconfirmed secret, so a re-scan works.
    It refuses with `409` when a *confirmed* authenticator already exists:
    replacing one is `DELETE /auth/mfa/totp` followed by this, and that
    needs the current password.

    This is the only response that ever carries the secret.

    Args:
        x_csrf_token (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TOTPEnrolment
    """

    return (
        await asyncio_detailed(
            client=client,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
