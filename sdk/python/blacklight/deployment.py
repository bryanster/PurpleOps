"""Connecting to a deployment: the part the OpenAPI document cannot say.

Everything else in this package is generated from ``api/openapi.yaml`` by
``make generate`` and must not be edited. This module and ``py.typed`` are the
two files here that are written by hand — the generator leaves files it did not
write alone, but it does overwrite ``__init__.py``, which is why ``connect`` is
imported from ``blacklight.deployment`` rather than from ``blacklight``.

Anything that *can* be expressed in the OpenAPI document belongs there instead.
A helper here is a second description of the API, and the point of this SDK is
that there is only one.
"""

from __future__ import annotations

from typing import Any

from .client import AuthenticatedClient, Client

__all__ = ["API_PATH", "connect"]

#: The prefix every operation in this SDK hangs off.
#:
#: The document declares its one server as the relative URL ``/api/v1``, because
#: the SPA is served from the same origin as the API and an absolute URL would
#: pin every deployment to one host. A generated client cannot send a request to
#: a relative URL, so the prefix is applied here instead.
API_PATH = "/api/v1"


def connect(
    base_url: str,
    token: str | None = None,
    **httpx_kwargs: Any,
) -> Client | AuthenticatedClient:
    """Return a client for the Blacklight deployment at ``base_url``.

    Args:
        base_url: The deployment's origin — ``https://blacklight.example.com`` —
            with no API path on it. :data:`API_PATH` is appended.
        token: A service token, the ``bl_<prefix>_<secret>`` string shown once
            when the token was created. It is sent as ``Authorization: Bearer``
            on every request. Without one the client is anonymous, which reaches
            only the handful of operations the document marks public.

            The other credential this API accepts is the browser session cookie,
            which this SDK deliberately does not help you obtain: a token can be
            scoped and expired by an administrator, and driving the login and
            MFA endpoints from a script to get a cookie instead is working
            around that.
        **httpx_kwargs: Passed through to the underlying client — ``timeout``,
            ``verify_ssl``, ``headers``, ``httpx_args`` and the rest.

    Raises:
        ValueError: if ``base_url`` is empty.
    """
    trimmed = base_url.strip()
    if not trimmed:
        raise ValueError(
            "blacklight: base_url is empty; pass the deployment origin, such as https://blacklight.example.com"
        )

    # Trailing slashes on both halves would produce `//api/v1`, which some
    # reverse proxies redirect and others 404.
    server = trimmed.rstrip("/") + API_PATH

    if token is None:
        return Client(base_url=server, **httpx_kwargs)
    if not token.strip():
        raise ValueError("blacklight: token is empty; pass None for an anonymous client")
    return AuthenticatedClient(base_url=server, token=token, **httpx_kwargs)
