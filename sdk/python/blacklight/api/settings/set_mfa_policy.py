from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.mfa_policy import MFAPolicy
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: MFAPolicy,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/settings/mfa",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> MFAPolicy | Problem | None:
    if response.status_code == 200:
        response_200 = MFAPolicy.from_dict(response.json())

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

    if response.status_code == 500:
        response_500 = Problem.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[MFAPolicy | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: MFAPolicy,
    x_csrf_token: str | Unset = UNSET,
) -> Response[MFAPolicy | Problem]:
    """Replace the platform-wide multi-factor authentication policy.

     Administrators only, and a whole replacement rather than a patch: both
    fields are required, so two administrators editing at once cannot each
    change the half they were thinking about and silently keep the other's.

    Turning a requirement **on** takes effect on the next request every
    signed-in user makes, not at their next sign-in. Somebody who is already
    signed in and has not enrolled is confined to enrolling; somebody who is
    enrolled but whose session never presented a factor is signed out and
    must sign in again (M1-008).

    Turning it **off** deletes nothing. Enrolments, recovery codes and the
    per-user `mfaEnforced` flag all survive, so switching it back on does not
    make everybody enrol again.

    Args:
        x_csrf_token (str | Unset):
        body (MFAPolicy): The platform-wide half of the multi-factor requirement (M1-008). The
            other half is the per-user `mfaEnforced` flag, and the effective
            requirement for one person is the **or** of whichever apply — so turning
            both of these off does not release somebody an administrator has
            individually enforced.

            Policy is evaluated before enrolment is looked at, which is the whole
            point: v1 asked "have they enrolled?" and so let anybody who skipped
            enrolment sign in with a password alone.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MFAPolicy | Problem]
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
    body: MFAPolicy,
    x_csrf_token: str | Unset = UNSET,
) -> MFAPolicy | Problem | None:
    """Replace the platform-wide multi-factor authentication policy.

     Administrators only, and a whole replacement rather than a patch: both
    fields are required, so two administrators editing at once cannot each
    change the half they were thinking about and silently keep the other's.

    Turning a requirement **on** takes effect on the next request every
    signed-in user makes, not at their next sign-in. Somebody who is already
    signed in and has not enrolled is confined to enrolling; somebody who is
    enrolled but whose session never presented a factor is signed out and
    must sign in again (M1-008).

    Turning it **off** deletes nothing. Enrolments, recovery codes and the
    per-user `mfaEnforced` flag all survive, so switching it back on does not
    make everybody enrol again.

    Args:
        x_csrf_token (str | Unset):
        body (MFAPolicy): The platform-wide half of the multi-factor requirement (M1-008). The
            other half is the per-user `mfaEnforced` flag, and the effective
            requirement for one person is the **or** of whichever apply — so turning
            both of these off does not release somebody an administrator has
            individually enforced.

            Policy is evaluated before enrolment is looked at, which is the whole
            point: v1 asked "have they enrolled?" and so let anybody who skipped
            enrolment sign in with a password alone.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MFAPolicy | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: MFAPolicy,
    x_csrf_token: str | Unset = UNSET,
) -> Response[MFAPolicy | Problem]:
    """Replace the platform-wide multi-factor authentication policy.

     Administrators only, and a whole replacement rather than a patch: both
    fields are required, so two administrators editing at once cannot each
    change the half they were thinking about and silently keep the other's.

    Turning a requirement **on** takes effect on the next request every
    signed-in user makes, not at their next sign-in. Somebody who is already
    signed in and has not enrolled is confined to enrolling; somebody who is
    enrolled but whose session never presented a factor is signed out and
    must sign in again (M1-008).

    Turning it **off** deletes nothing. Enrolments, recovery codes and the
    per-user `mfaEnforced` flag all survive, so switching it back on does not
    make everybody enrol again.

    Args:
        x_csrf_token (str | Unset):
        body (MFAPolicy): The platform-wide half of the multi-factor requirement (M1-008). The
            other half is the per-user `mfaEnforced` flag, and the effective
            requirement for one person is the **or** of whichever apply — so turning
            both of these off does not release somebody an administrator has
            individually enforced.

            Policy is evaluated before enrolment is looked at, which is the whole
            point: v1 asked "have they enrolled?" and so let anybody who skipped
            enrolment sign in with a password alone.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MFAPolicy | Problem]
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
    body: MFAPolicy,
    x_csrf_token: str | Unset = UNSET,
) -> MFAPolicy | Problem | None:
    """Replace the platform-wide multi-factor authentication policy.

     Administrators only, and a whole replacement rather than a patch: both
    fields are required, so two administrators editing at once cannot each
    change the half they were thinking about and silently keep the other's.

    Turning a requirement **on** takes effect on the next request every
    signed-in user makes, not at their next sign-in. Somebody who is already
    signed in and has not enrolled is confined to enrolling; somebody who is
    enrolled but whose session never presented a factor is signed out and
    must sign in again (M1-008).

    Turning it **off** deletes nothing. Enrolments, recovery codes and the
    per-user `mfaEnforced` flag all survive, so switching it back on does not
    make everybody enrol again.

    Args:
        x_csrf_token (str | Unset):
        body (MFAPolicy): The platform-wide half of the multi-factor requirement (M1-008). The
            other half is the per-user `mfaEnforced` flag, and the effective
            requirement for one person is the **or** of whichever apply — so turning
            both of these off does not release somebody an administrator has
            individually enforced.

            Policy is evaluated before enrolment is looked at, which is the whole
            point: v1 asked "have they enrolled?" and so let anybody who skipped
            enrolment sign in with a password alone.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MFAPolicy | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
