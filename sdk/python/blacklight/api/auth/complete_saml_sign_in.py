from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.complete_saml_sign_in_body import CompleteSamlSignInBody
from ...models.problem import Problem
from typing import cast


def _get_kwargs(
    *,
    body: CompleteSamlSignInBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/saml/acs",
    }

    _kwargs["data"] = body.to_dict()
    headers["Content-Type"] = "application/x-www-form-urlencoded"

    _kwargs["headers"] = headers
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
    body: CompleteSamlSignInBody,
) -> Response[Any | Problem]:
    """Consume a SAML assertion and issue a session.

     The assertion consumer service. The identity provider POSTs a form here
    from the browser; register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/acs` as the ACS URL, and it must
    match byte for byte — it is checked against the assertion's `Recipient`
    and the response's `Destination`.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to the path the sign-in named, or to `/`.

    Both SP-initiated and IdP-initiated sign-in are accepted. An
    SP-initiated one is additionally bound to the browser that started it —
    the sealed cookie names the request ID the assertion must answer, so an
    assertion delivered into somebody else's browser is refused. An
    IdP-initiated one has no request to answer and so cannot have that
    binding; set `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED=false` on a deployment
    that does not need it.

    The failures, and none of them is a redirect:

    - `401` — the assertion was rejected. Unsigned, signed by a key that is
      not the provider's, tampered with, outside its validity window,
      addressed to somebody else, or one that has already been used. Which
      of those it was is in the log and never in the response.
    - `403` — the assertion was good and this deployment will not sign that
      person in: no account and `BLACKLIGHT_SAML_AUTO_PROVISION` is off, or
      the account is disabled.
    - `404` — no provider is configured.

    A second factor still applies, exactly as it does for OIDC (M1-006,
    M1-008).

    Args:
        body (CompleteSamlSignInBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: CompleteSamlSignInBody,
) -> Any | Problem | None:
    """Consume a SAML assertion and issue a session.

     The assertion consumer service. The identity provider POSTs a form here
    from the browser; register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/acs` as the ACS URL, and it must
    match byte for byte — it is checked against the assertion's `Recipient`
    and the response's `Destination`.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to the path the sign-in named, or to `/`.

    Both SP-initiated and IdP-initiated sign-in are accepted. An
    SP-initiated one is additionally bound to the browser that started it —
    the sealed cookie names the request ID the assertion must answer, so an
    assertion delivered into somebody else's browser is refused. An
    IdP-initiated one has no request to answer and so cannot have that
    binding; set `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED=false` on a deployment
    that does not need it.

    The failures, and none of them is a redirect:

    - `401` — the assertion was rejected. Unsigned, signed by a key that is
      not the provider's, tampered with, outside its validity window,
      addressed to somebody else, or one that has already been used. Which
      of those it was is in the log and never in the response.
    - `403` — the assertion was good and this deployment will not sign that
      person in: no account and `BLACKLIGHT_SAML_AUTO_PROVISION` is off, or
      the account is disabled.
    - `404` — no provider is configured.

    A second factor still applies, exactly as it does for OIDC (M1-006,
    M1-008).

    Args:
        body (CompleteSamlSignInBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: CompleteSamlSignInBody,
) -> Response[Any | Problem]:
    """Consume a SAML assertion and issue a session.

     The assertion consumer service. The identity provider POSTs a form here
    from the browser; register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/acs` as the ACS URL, and it must
    match byte for byte — it is checked against the assertion's `Recipient`
    and the response's `Destination`.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to the path the sign-in named, or to `/`.

    Both SP-initiated and IdP-initiated sign-in are accepted. An
    SP-initiated one is additionally bound to the browser that started it —
    the sealed cookie names the request ID the assertion must answer, so an
    assertion delivered into somebody else's browser is refused. An
    IdP-initiated one has no request to answer and so cannot have that
    binding; set `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED=false` on a deployment
    that does not need it.

    The failures, and none of them is a redirect:

    - `401` — the assertion was rejected. Unsigned, signed by a key that is
      not the provider's, tampered with, outside its validity window,
      addressed to somebody else, or one that has already been used. Which
      of those it was is in the log and never in the response.
    - `403` — the assertion was good and this deployment will not sign that
      person in: no account and `BLACKLIGHT_SAML_AUTO_PROVISION` is off, or
      the account is disabled.
    - `404` — no provider is configured.

    A second factor still applies, exactly as it does for OIDC (M1-006,
    M1-008).

    Args:
        body (CompleteSamlSignInBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: CompleteSamlSignInBody,
) -> Any | Problem | None:
    """Consume a SAML assertion and issue a session.

     The assertion consumer service. The identity provider POSTs a form here
    from the browser; register
    `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/acs` as the ACS URL, and it must
    match byte for byte — it is checked against the assertion's `Recipient`
    and the response's `Destination`.

    On success this behaves exactly like `POST /auth/login` with `status:
    authenticated`: the session cookie and the `bl_csrf` cookie are set, and
    the browser is redirected to the path the sign-in named, or to `/`.

    Both SP-initiated and IdP-initiated sign-in are accepted. An
    SP-initiated one is additionally bound to the browser that started it —
    the sealed cookie names the request ID the assertion must answer, so an
    assertion delivered into somebody else's browser is refused. An
    IdP-initiated one has no request to answer and so cannot have that
    binding; set `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED=false` on a deployment
    that does not need it.

    The failures, and none of them is a redirect:

    - `401` — the assertion was rejected. Unsigned, signed by a key that is
      not the provider's, tampered with, outside its validity window,
      addressed to somebody else, or one that has already been used. Which
      of those it was is in the log and never in the response.
    - `403` — the assertion was good and this deployment will not sign that
      person in: no account and `BLACKLIGHT_SAML_AUTO_PROVISION` is off, or
      the account is disabled.
    - `404` — no provider is configured.

    A second factor still applies, exactly as it does for OIDC (M1-006,
    M1-008).

    Args:
        body (CompleteSamlSignInBody):

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
        )
    ).parsed
