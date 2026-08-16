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
    *,
    code: str | Unset = UNSET,
    state: str | Unset = UNSET,
    error: str | Unset = UNSET,
    error_description: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["code"] = code

    params["state"] = state

    params["error"] = error

    params["error_description"] = error_description

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/oidc/callback",
        "params": params,
    }

    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Any | Problem | None:
    if response.status_code == 302:
        response_302 = cast(Any, None)
        return response_302

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
    code: str | Unset = UNSET,
    state: str | Unset = UNSET,
    error: str | Unset = UNSET,
    error_description: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Complete a single sign-on and issue a session.

     Where the identity provider sends the browser back. Register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback` as the redirect URI at
    the provider; it must match byte for byte.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to `return_to` — or to `/` when the sign-in
    named none. The pending-state cookie is cleared, which is what makes a
    `state` single-use.

    The failures are worth telling apart, and none of them is a redirect:

    - `401` — the callback does not belong to a sign-in this browser started
      (no cookie, a `state` that does not match, an expired one), or the ID
      token did not verify. Start again.
    - `403` — the provider vouched for somebody this deployment has no
      account for and `BLACKLIGHT_OIDC_AUTO_PROVISION` is off, or the account
      is disabled. The message says which and what to do about it; no account
      is created.
    - `404` — no provider is configured.

    A second factor still applies. An account that also has a local password
    and a confirmed authenticator is answered `mfa_required` in the same way
    a local sign-in is, and lands on the code entry page rather than signed
    in (M1-006, M1-008).

    Args:
        code (str | Unset):
        state (str | Unset):
        error (str | Unset):
        error_description (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        code=code,
        state=state,
        error=error,
        error_description=error_description,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    code: str | Unset = UNSET,
    state: str | Unset = UNSET,
    error: str | Unset = UNSET,
    error_description: str | Unset = UNSET,
) -> Any | Problem | None:
    """Complete a single sign-on and issue a session.

     Where the identity provider sends the browser back. Register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback` as the redirect URI at
    the provider; it must match byte for byte.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to `return_to` — or to `/` when the sign-in
    named none. The pending-state cookie is cleared, which is what makes a
    `state` single-use.

    The failures are worth telling apart, and none of them is a redirect:

    - `401` — the callback does not belong to a sign-in this browser started
      (no cookie, a `state` that does not match, an expired one), or the ID
      token did not verify. Start again.
    - `403` — the provider vouched for somebody this deployment has no
      account for and `BLACKLIGHT_OIDC_AUTO_PROVISION` is off, or the account
      is disabled. The message says which and what to do about it; no account
      is created.
    - `404` — no provider is configured.

    A second factor still applies. An account that also has a local password
    and a confirmed authenticator is answered `mfa_required` in the same way
    a local sign-in is, and lands on the code entry page rather than signed
    in (M1-006, M1-008).

    Args:
        code (str | Unset):
        state (str | Unset):
        error (str | Unset):
        error_description (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        code=code,
        state=state,
        error=error,
        error_description=error_description,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    code: str | Unset = UNSET,
    state: str | Unset = UNSET,
    error: str | Unset = UNSET,
    error_description: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """Complete a single sign-on and issue a session.

     Where the identity provider sends the browser back. Register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback` as the redirect URI at
    the provider; it must match byte for byte.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to `return_to` — or to `/` when the sign-in
    named none. The pending-state cookie is cleared, which is what makes a
    `state` single-use.

    The failures are worth telling apart, and none of them is a redirect:

    - `401` — the callback does not belong to a sign-in this browser started
      (no cookie, a `state` that does not match, an expired one), or the ID
      token did not verify. Start again.
    - `403` — the provider vouched for somebody this deployment has no
      account for and `BLACKLIGHT_OIDC_AUTO_PROVISION` is off, or the account
      is disabled. The message says which and what to do about it; no account
      is created.
    - `404` — no provider is configured.

    A second factor still applies. An account that also has a local password
    and a confirmed authenticator is answered `mfa_required` in the same way
    a local sign-in is, and lands on the code entry page rather than signed
    in (M1-006, M1-008).

    Args:
        code (str | Unset):
        state (str | Unset):
        error (str | Unset):
        error_description (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        code=code,
        state=state,
        error=error,
        error_description=error_description,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    code: str | Unset = UNSET,
    state: str | Unset = UNSET,
    error: str | Unset = UNSET,
    error_description: str | Unset = UNSET,
) -> Any | Problem | None:
    """Complete a single sign-on and issue a session.

     Where the identity provider sends the browser back. Register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback` as the redirect URI at
    the provider; it must match byte for byte.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to `return_to` — or to `/` when the sign-in
    named none. The pending-state cookie is cleared, which is what makes a
    `state` single-use.

    The failures are worth telling apart, and none of them is a redirect:

    - `401` — the callback does not belong to a sign-in this browser started
      (no cookie, a `state` that does not match, an expired one), or the ID
      token did not verify. Start again.
    - `403` — the provider vouched for somebody this deployment has no
      account for and `BLACKLIGHT_OIDC_AUTO_PROVISION` is off, or the account
      is disabled. The message says which and what to do about it; no account
      is created.
    - `404` — no provider is configured.

    A second factor still applies. An account that also has a local password
    and a confirmed authenticator is answered `mfa_required` in the same way
    a local sign-in is, and lands on the code entry page rather than signed
    in (M1-006, M1-008).

    Args:
        code (str | Unset):
        state (str | Unset):
        error (str | Unset):
        error_description (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            code=code,
            state=state,
            error=error,
            error_description=error_description,
        )
    ).parsed
