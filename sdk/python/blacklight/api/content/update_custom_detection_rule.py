from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_detection_rule import ContentDetectionRule
from ...models.problem import Problem
from ...models.update_custom_detection_rule_request import UpdateCustomDetectionRuleRequest
from ...types import UNSET, Unset
from typing import cast
from uuid import UUID


def _get_kwargs(
    rule_id: UUID,
    *,
    body: UpdateCustomDetectionRuleRequest,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/content/custom/detection-rules/{rule_id}".format(
            rule_id=quote(str(rule_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentDetectionRule | Problem | None:
    if response.status_code == 200:
        response_200 = ContentDetectionRule.from_dict(response.json())

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
) -> Response[ContentDetectionRule | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateCustomDetectionRuleRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentDetectionRule | Problem]:
    """Update a custom detection rule reference.

    Args:
        rule_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateCustomDetectionRuleRequest): Partial patch for a custom detection rule
            reference.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentDetectionRule | Problem]
    """

    kwargs = _get_kwargs(
        rule_id=rule_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateCustomDetectionRuleRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentDetectionRule | Problem | None:
    """Update a custom detection rule reference.

    Args:
        rule_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateCustomDetectionRuleRequest): Partial patch for a custom detection rule
            reference.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentDetectionRule | Problem
    """

    return sync_detailed(
        rule_id=rule_id,
        client=client,
        body=body,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateCustomDetectionRuleRequest,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentDetectionRule | Problem]:
    """Update a custom detection rule reference.

    Args:
        rule_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateCustomDetectionRuleRequest): Partial patch for a custom detection rule
            reference.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentDetectionRule | Problem]
    """

    kwargs = _get_kwargs(
        rule_id=rule_id,
        body=body,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateCustomDetectionRuleRequest,
    x_csrf_token: str | Unset = UNSET,
) -> ContentDetectionRule | Problem | None:
    """Update a custom detection rule reference.

    Args:
        rule_id (UUID):
        x_csrf_token (str | Unset):
        body (UpdateCustomDetectionRuleRequest): Partial patch for a custom detection rule
            reference.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentDetectionRule | Problem
    """

    return (
        await asyncio_detailed(
            rule_id=rule_id,
            client=client,
            body=body,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
