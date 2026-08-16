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
    topics: list[str] | Unset = UNSET,
    last_event_id_query: str | Unset = UNSET,
    last_event_id_header: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(last_event_id_header, Unset):
        headers["Last-Event-ID"] = last_event_id_header

    params: dict[str, Any] = {}

    json_topics: list[str] | Unset = UNSET
    if not isinstance(topics, Unset):
        json_topics = topics

    params["topics"] = json_topics

    params["lastEventId"] = last_event_id_query

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/events",
        "params": params,
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Problem | str | None:
    if response.status_code == 200:
        response_200 = response.text
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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Problem | str]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    topics: list[str] | Unset = UNSET,
    last_event_id_query: str | Unset = UNSET,
    last_event_id_header: str | Unset = UNSET,
) -> Response[Problem | str]:
    """Subscribe to server-sent events.

     Opens a long-lived `text/event-stream` for live UI updates. M2 streams
    content sync job progress; M4 extends the same hub with engagement
    topics.

    **Session cookie only.** Service tokens are refused — EventSource
    cannot attach an Authorization header, and a long-lived
    token-authenticated stream is not a browser subscription. The
    operation's `security` lists only `cookieSession`, and the
    authorization middleware refuses `MethodServiceToken`.

    Query `topics` is repeatable. Per-topic authorization is enforced in
    the SSE handler (not in the middleware — this endpoint maps to a
    self-service operation so any authenticated session may subscribe,
    then TopicAuthz filters per topic). Unknown topic names are `400`
    (never silently widened). Topics:

    - `content.jobs` / `content.jobs.{jobId}` — requires
      `content.sync` (administrators)
    - `engagement.{engagementId}` — requires `engagement.read`
      (members and administrators of that engagement)

    Event `type` values are stable: `content.job.progress` while a job runs,
    `content.job.terminal` once when it ends. Each frame carries `id`
    (UUIDv7), `event` (the type), and a JSON `data` payload.

    `Last-Event-ID` is accepted but **best-effort only in M2**: there is no
    replay against the activity log yet. On reconnect, the SPA should
    reconcile from `GET /content/jobs/{id}`. M4 owns guaranteed catch-up.

    Heartbeat comment frames (`: ping`) are written periodically so idle
    reverse proxies do not close the connection. Operators must disable
    response buffering on this path — see `docs/deploy.md`.

    Args:
        topics (list[str] | Unset):
        last_event_id_query (str | Unset):
        last_event_id_header (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        topics=topics,
        last_event_id_query=last_event_id_query,
        last_event_id_header=last_event_id_header,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    topics: list[str] | Unset = UNSET,
    last_event_id_query: str | Unset = UNSET,
    last_event_id_header: str | Unset = UNSET,
) -> Problem | str | None:
    """Subscribe to server-sent events.

     Opens a long-lived `text/event-stream` for live UI updates. M2 streams
    content sync job progress; M4 extends the same hub with engagement
    topics.

    **Session cookie only.** Service tokens are refused — EventSource
    cannot attach an Authorization header, and a long-lived
    token-authenticated stream is not a browser subscription. The
    operation's `security` lists only `cookieSession`, and the
    authorization middleware refuses `MethodServiceToken`.

    Query `topics` is repeatable. Per-topic authorization is enforced in
    the SSE handler (not in the middleware — this endpoint maps to a
    self-service operation so any authenticated session may subscribe,
    then TopicAuthz filters per topic). Unknown topic names are `400`
    (never silently widened). Topics:

    - `content.jobs` / `content.jobs.{jobId}` — requires
      `content.sync` (administrators)
    - `engagement.{engagementId}` — requires `engagement.read`
      (members and administrators of that engagement)

    Event `type` values are stable: `content.job.progress` while a job runs,
    `content.job.terminal` once when it ends. Each frame carries `id`
    (UUIDv7), `event` (the type), and a JSON `data` payload.

    `Last-Event-ID` is accepted but **best-effort only in M2**: there is no
    replay against the activity log yet. On reconnect, the SPA should
    reconcile from `GET /content/jobs/{id}`. M4 owns guaranteed catch-up.

    Heartbeat comment frames (`: ping`) are written periodically so idle
    reverse proxies do not close the connection. Operators must disable
    response buffering on this path — see `docs/deploy.md`.

    Args:
        topics (list[str] | Unset):
        last_event_id_query (str | Unset):
        last_event_id_header (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return sync_detailed(
        client=client,
        topics=topics,
        last_event_id_query=last_event_id_query,
        last_event_id_header=last_event_id_header,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    topics: list[str] | Unset = UNSET,
    last_event_id_query: str | Unset = UNSET,
    last_event_id_header: str | Unset = UNSET,
) -> Response[Problem | str]:
    """Subscribe to server-sent events.

     Opens a long-lived `text/event-stream` for live UI updates. M2 streams
    content sync job progress; M4 extends the same hub with engagement
    topics.

    **Session cookie only.** Service tokens are refused — EventSource
    cannot attach an Authorization header, and a long-lived
    token-authenticated stream is not a browser subscription. The
    operation's `security` lists only `cookieSession`, and the
    authorization middleware refuses `MethodServiceToken`.

    Query `topics` is repeatable. Per-topic authorization is enforced in
    the SSE handler (not in the middleware — this endpoint maps to a
    self-service operation so any authenticated session may subscribe,
    then TopicAuthz filters per topic). Unknown topic names are `400`
    (never silently widened). Topics:

    - `content.jobs` / `content.jobs.{jobId}` — requires
      `content.sync` (administrators)
    - `engagement.{engagementId}` — requires `engagement.read`
      (members and administrators of that engagement)

    Event `type` values are stable: `content.job.progress` while a job runs,
    `content.job.terminal` once when it ends. Each frame carries `id`
    (UUIDv7), `event` (the type), and a JSON `data` payload.

    `Last-Event-ID` is accepted but **best-effort only in M2**: there is no
    replay against the activity log yet. On reconnect, the SPA should
    reconcile from `GET /content/jobs/{id}`. M4 owns guaranteed catch-up.

    Heartbeat comment frames (`: ping`) are written periodically so idle
    reverse proxies do not close the connection. Operators must disable
    response buffering on this path — see `docs/deploy.md`.

    Args:
        topics (list[str] | Unset):
        last_event_id_query (str | Unset):
        last_event_id_header (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | str]
    """

    kwargs = _get_kwargs(
        topics=topics,
        last_event_id_query=last_event_id_query,
        last_event_id_header=last_event_id_header,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    topics: list[str] | Unset = UNSET,
    last_event_id_query: str | Unset = UNSET,
    last_event_id_header: str | Unset = UNSET,
) -> Problem | str | None:
    """Subscribe to server-sent events.

     Opens a long-lived `text/event-stream` for live UI updates. M2 streams
    content sync job progress; M4 extends the same hub with engagement
    topics.

    **Session cookie only.** Service tokens are refused — EventSource
    cannot attach an Authorization header, and a long-lived
    token-authenticated stream is not a browser subscription. The
    operation's `security` lists only `cookieSession`, and the
    authorization middleware refuses `MethodServiceToken`.

    Query `topics` is repeatable. Per-topic authorization is enforced in
    the SSE handler (not in the middleware — this endpoint maps to a
    self-service operation so any authenticated session may subscribe,
    then TopicAuthz filters per topic). Unknown topic names are `400`
    (never silently widened). Topics:

    - `content.jobs` / `content.jobs.{jobId}` — requires
      `content.sync` (administrators)
    - `engagement.{engagementId}` — requires `engagement.read`
      (members and administrators of that engagement)

    Event `type` values are stable: `content.job.progress` while a job runs,
    `content.job.terminal` once when it ends. Each frame carries `id`
    (UUIDv7), `event` (the type), and a JSON `data` payload.

    `Last-Event-ID` is accepted but **best-effort only in M2**: there is no
    replay against the activity log yet. On reconnect, the SPA should
    reconcile from `GET /content/jobs/{id}`. M4 owns guaranteed catch-up.

    Heartbeat comment frames (`: ping`) are written periodically so idle
    reverse proxies do not close the connection. Operators must disable
    response buffering on this path — see `docs/deploy.md`.

    Args:
        topics (list[str] | Unset):
        last_event_id_query (str | Unset):
        last_event_id_header (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | str
    """

    return (
        await asyncio_detailed(
            client=client,
            topics=topics,
            last_event_id_query=last_event_id_query,
            last_event_id_header=last_event_id_header,
        )
    ).parsed
