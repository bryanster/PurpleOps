from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.content_import_report import ContentImportReport
from ...models.content_sync_job import ContentSyncJob
from ...models.import_custom_content_request import ImportCustomContentRequest
from ...models.problem import Problem
from ...types import UNSET, Unset
from typing import cast


def _get_kwargs(
    *,
    body: ImportCustomContentRequest,
    dry_run: bool | Unset = False,
    fail_fast: bool | Unset = False,
    x_csrf_token: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_csrf_token, Unset):
        headers["X-CSRF-Token"] = x_csrf_token

    params: dict[str, Any] = {}

    params["dryRun"] = dry_run

    params["failFast"] = fail_fast

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/content/custom/import",
        "params": params,
    }

    _kwargs["files"] = body.to_multipart()

    headers["Content-Type"] = "multipart/form-data; boundary=+++"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContentImportReport | ContentSyncJob | Problem | None:
    if response.status_code == 200:
        response_200 = ContentImportReport.from_dict(response.json())

        return response_200

    if response.status_code == 202:
        response_202 = ContentSyncJob.from_dict(response.json())

        return response_202

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
) -> Response[ContentImportReport | ContentSyncJob | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ImportCustomContentRequest,
    dry_run: bool | Unset = False,
    fail_fast: bool | Unset = False,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentImportReport | ContentSyncJob | Problem]:
    """Import v1 custom testcases or knowledgebase files.

     Administrators only (`content.manage`). Accepts a multipart upload of
    v1-shaped content and upserts rows under the singleton `custom` source:

    - `testcases_json` — v1 `custom/testcases.json` (array or `{testcases:[…]}`)
    - `testcases_yaml` — one YAML file or a zip of `*.yaml` testcase files
      (the layout the v1 seeder globbed as `custom/testcases/*.yaml`)
    - `knowledgebase_yaml` — one YAML file or a zip of KB notes
    - `auto` — sniff JSON testcases, KB yaml, testcase yaml, custom export,
      or a zip of the above

    Small uploads run synchronously and answer `200` with counts/warnings.
    Uploads over the sync threshold (1 MiB) enqueue a `v1_import` job and
    answer `202` (same global job slot as content sync). `dryRun=true`
    always runs synchronously and never writes.

    Partial file failures are reported per path and do not abort the rest
    unless `failFast=true`. Re-import is idempotent: external ids are
    derived deterministically from v1 ids/names (see `docs/content-v1-import.md`).

    Args:
        dry_run (bool | Unset):  Default: False.
        fail_fast (bool | Unset):  Default: False.
        x_csrf_token (str | Unset):
        body (ImportCustomContentRequest): Multipart v1 custom import. `file` is a single
            JSON/YAML document or a
            zip of files; `format` selects the parser (or `auto` to sniff).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentImportReport | ContentSyncJob | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        dry_run=dry_run,
        fail_fast=fail_fast,
        x_csrf_token=x_csrf_token,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: ImportCustomContentRequest,
    dry_run: bool | Unset = False,
    fail_fast: bool | Unset = False,
    x_csrf_token: str | Unset = UNSET,
) -> ContentImportReport | ContentSyncJob | Problem | None:
    """Import v1 custom testcases or knowledgebase files.

     Administrators only (`content.manage`). Accepts a multipart upload of
    v1-shaped content and upserts rows under the singleton `custom` source:

    - `testcases_json` — v1 `custom/testcases.json` (array or `{testcases:[…]}`)
    - `testcases_yaml` — one YAML file or a zip of `*.yaml` testcase files
      (the layout the v1 seeder globbed as `custom/testcases/*.yaml`)
    - `knowledgebase_yaml` — one YAML file or a zip of KB notes
    - `auto` — sniff JSON testcases, KB yaml, testcase yaml, custom export,
      or a zip of the above

    Small uploads run synchronously and answer `200` with counts/warnings.
    Uploads over the sync threshold (1 MiB) enqueue a `v1_import` job and
    answer `202` (same global job slot as content sync). `dryRun=true`
    always runs synchronously and never writes.

    Partial file failures are reported per path and do not abort the rest
    unless `failFast=true`. Re-import is idempotent: external ids are
    derived deterministically from v1 ids/names (see `docs/content-v1-import.md`).

    Args:
        dry_run (bool | Unset):  Default: False.
        fail_fast (bool | Unset):  Default: False.
        x_csrf_token (str | Unset):
        body (ImportCustomContentRequest): Multipart v1 custom import. `file` is a single
            JSON/YAML document or a
            zip of files; `format` selects the parser (or `auto` to sniff).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentImportReport | ContentSyncJob | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        dry_run=dry_run,
        fail_fast=fail_fast,
        x_csrf_token=x_csrf_token,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ImportCustomContentRequest,
    dry_run: bool | Unset = False,
    fail_fast: bool | Unset = False,
    x_csrf_token: str | Unset = UNSET,
) -> Response[ContentImportReport | ContentSyncJob | Problem]:
    """Import v1 custom testcases or knowledgebase files.

     Administrators only (`content.manage`). Accepts a multipart upload of
    v1-shaped content and upserts rows under the singleton `custom` source:

    - `testcases_json` — v1 `custom/testcases.json` (array or `{testcases:[…]}`)
    - `testcases_yaml` — one YAML file or a zip of `*.yaml` testcase files
      (the layout the v1 seeder globbed as `custom/testcases/*.yaml`)
    - `knowledgebase_yaml` — one YAML file or a zip of KB notes
    - `auto` — sniff JSON testcases, KB yaml, testcase yaml, custom export,
      or a zip of the above

    Small uploads run synchronously and answer `200` with counts/warnings.
    Uploads over the sync threshold (1 MiB) enqueue a `v1_import` job and
    answer `202` (same global job slot as content sync). `dryRun=true`
    always runs synchronously and never writes.

    Partial file failures are reported per path and do not abort the rest
    unless `failFast=true`. Re-import is idempotent: external ids are
    derived deterministically from v1 ids/names (see `docs/content-v1-import.md`).

    Args:
        dry_run (bool | Unset):  Default: False.
        fail_fast (bool | Unset):  Default: False.
        x_csrf_token (str | Unset):
        body (ImportCustomContentRequest): Multipart v1 custom import. `file` is a single
            JSON/YAML document or a
            zip of files; `format` selects the parser (or `auto` to sniff).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContentImportReport | ContentSyncJob | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        dry_run=dry_run,
        fail_fast=fail_fast,
        x_csrf_token=x_csrf_token,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: ImportCustomContentRequest,
    dry_run: bool | Unset = False,
    fail_fast: bool | Unset = False,
    x_csrf_token: str | Unset = UNSET,
) -> ContentImportReport | ContentSyncJob | Problem | None:
    """Import v1 custom testcases or knowledgebase files.

     Administrators only (`content.manage`). Accepts a multipart upload of
    v1-shaped content and upserts rows under the singleton `custom` source:

    - `testcases_json` — v1 `custom/testcases.json` (array or `{testcases:[…]}`)
    - `testcases_yaml` — one YAML file or a zip of `*.yaml` testcase files
      (the layout the v1 seeder globbed as `custom/testcases/*.yaml`)
    - `knowledgebase_yaml` — one YAML file or a zip of KB notes
    - `auto` — sniff JSON testcases, KB yaml, testcase yaml, custom export,
      or a zip of the above

    Small uploads run synchronously and answer `200` with counts/warnings.
    Uploads over the sync threshold (1 MiB) enqueue a `v1_import` job and
    answer `202` (same global job slot as content sync). `dryRun=true`
    always runs synchronously and never writes.

    Partial file failures are reported per path and do not abort the rest
    unless `failFast=true`. Re-import is idempotent: external ids are
    derived deterministically from v1 ids/names (see `docs/content-v1-import.md`).

    Args:
        dry_run (bool | Unset):  Default: False.
        fail_fast (bool | Unset):  Default: False.
        x_csrf_token (str | Unset):
        body (ImportCustomContentRequest): Multipart v1 custom import. `file` is a single
            JSON/YAML document or a
            zip of files; `format` selects the parser (or `auto` to sniff).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContentImportReport | ContentSyncJob | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            dry_run=dry_run,
            fail_fast=fail_fast,
            x_csrf_token=x_csrf_token,
        )
    ).parsed
