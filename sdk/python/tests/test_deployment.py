"""What these cover is the seam between the hand-written helper and the
generated client: where a request goes, what it carries, and how a documented
failure arrives.

They do not re-test the generator. Whether ``list_engagements`` serialises its
query parameters correctly is openapi-python-client's business, and asserting it
here would mean writing the request builder out a second time by hand.
"""

import httpx
import pytest
import respx

from blacklight.api.system.get_health import sync_detailed as get_health
from blacklight.deployment import API_PATH, connect
from blacklight.models.health_state import HealthState

ORIGIN = "https://blacklight.example.com"
HEALTHZ = f"{ORIGIN}{API_PATH}/healthz"

HEALTHY = {"status": "ok", "checks": {"db": "ok"}}


def test_connect_appends_the_api_path() -> None:
    """The reason connect() exists: the document's one server is the relative
    URL /api/v1, so a caller who passed their origin to the generated client
    would be talking to the SPA's index.html."""
    with respx.mock:
        route = respx.get(HEALTHZ).mock(return_value=httpx.Response(200, json=HEALTHY))

        get_health(client=connect(ORIGIN))

        assert route.called


def test_connect_tolerates_a_trailing_slash() -> None:
    """An operator's BLACKLIGHT_URL very often ends in one, and `//api/v1` is
    redirected by some reverse proxies and 404ed by others."""
    with respx.mock:
        route = respx.get(HEALTHZ).mock(return_value=httpx.Response(200, json=HEALTHY))

        get_health(client=connect(ORIGIN + "///"))

        assert route.called


def test_connect_refuses_an_empty_base_url() -> None:
    with pytest.raises(ValueError):
        connect("   ")


def test_connect_refuses_an_empty_token() -> None:
    """An empty string is not "no token": it would produce `Bearer ` and reach
    the server as an anonymous call, which fails as a 401 far from the mistake."""
    with pytest.raises(ValueError):
        connect(ORIGIN, token="  ")


def test_a_token_is_sent_as_a_bearer_credential() -> None:
    with respx.mock:
        route = respx.get(HEALTHZ).mock(return_value=httpx.Response(200, json=HEALTHY))

        get_health(client=connect(ORIGIN, token="bl_abcd_secret"))

        assert route.calls.last.request.headers["authorization"] == "Bearer bl_abcd_secret"


def test_no_token_sends_no_credential() -> None:
    with respx.mock:
        route = respx.get(HEALTHZ).mock(return_value=httpx.Response(200, json=HEALTHY))

        get_health(client=connect(ORIGIN))

        assert "authorization" not in route.calls.last.request.headers


def test_a_documented_status_parses_into_its_own_model() -> None:
    """The whole argument for a generated client over hand-written requests:
    /healthz answers 503 with the same Health shape as its 200 — the interesting
    part being which dependency is down — and the caller does not work out which
    it got by reading the bytes."""
    unhealthy = {"status": "error", "checks": {"db": "error"}}

    with respx.mock:
        respx.get(HEALTHZ).mock(return_value=httpx.Response(503, json=unhealthy))

        response = get_health(client=connect(ORIGIN))

    assert response.status_code == 503
    assert response.parsed is not None
    assert response.parsed.status == HealthState.ERROR
    assert response.parsed.checks.db == HealthState.ERROR
