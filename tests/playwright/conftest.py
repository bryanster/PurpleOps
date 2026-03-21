"""Global fixtures for PurpleOps Playwright UI tests."""

import uuid
import os
import tempfile

import pytest
from playwright.sync_api import Page, Browser, expect

from pages.login_page import LoginPage
from pages.assessments_page import AssessmentsPage
from pages.assessment_detail_page import AssessmentDetailPage
from pages.access_page import AccessPage

BASE_URL = os.environ.get("PURPLEOPS_BASE_URL", "http://localhost:8888")
ADMIN_EMAIL = "admin@purpleops.com"
ADMIN_PASSWORD = os.environ.get("PURPLEOPS_ADMIN_PWD", "TestAdmin123!")


@pytest.fixture(scope="session")
def base_url():
    return BASE_URL


@pytest.fixture
def admin_credentials():
    return {"email": ADMIN_EMAIL, "password": ADMIN_PASSWORD}


def _do_login(page: Page, email: str, password: str):
    """Perform login and handle initial password change redirect."""
    login_page = LoginPage(page)
    login_page.login(email, password)
    page.wait_for_timeout(1000)
    # Handle initial password change redirect
    if "/password/change" in page.url:
        page.goto(f"{BASE_URL}/password/changed")
        page.wait_for_timeout(500)
    # Handle MFA redirect
    if "/mfa/" in page.url:
        pytest.skip("MFA is enabled; test requires non-MFA login")


@pytest.fixture
def authenticated_page(page: Page):
    """A page logged in as admin."""
    page.goto(f"{BASE_URL}/login")
    _do_login(page, ADMIN_EMAIL, ADMIN_PASSWORD)
    return page


@pytest.fixture
def admin_page(authenticated_page: Page):
    """Alias for authenticated_page (admin role)."""
    return authenticated_page


def _create_user_via_ui(
    admin_page: Page,
    email: str,
    username: str,
    password: str,
    roles: list[str],
    assessments: list[str] = None,
):
    """Create a user via the Access Control UI using an admin session."""
    access = AccessPage(admin_page)
    access.navigate()
    access.create_user(
        email=email,
        username=username,
        password=password,
        roles=roles,
        assessments=assessments,
    )


def _delete_user_via_ui(admin_page: Page, username: str):
    """Delete a user via the Access Control UI."""
    access = AccessPage(admin_page)
    access.navigate()
    row_idx = access.find_user_row(username)
    if row_idx >= 0:
        access.delete_user_by_row(row_idx)


@pytest.fixture
def create_test_user(authenticated_page: Page):
    """Factory fixture to create test users. Returns credentials dict.
    Users are cleaned up after the test."""
    created_users = []

    def _create(roles: list[str], assessments: list[str] = None):
        uid = uuid.uuid4().hex[:8]
        creds = {
            "email": f"test-{uid}@purpleops.com",
            "username": f"testuser-{uid}",
            "password": "TestPassword123!",
        }
        _create_user_via_ui(
            authenticated_page,
            email=creds["email"],
            username=creds["username"],
            password=creds["password"],
            roles=roles,
            assessments=assessments,
        )
        created_users.append(creds["username"])
        return creds

    yield _create

    # Cleanup
    for username in created_users:
        try:
            _delete_user_via_ui(authenticated_page, username)
        except Exception:
            pass


def _login_as_user(browser: Browser, email: str, password: str) -> Page:
    """Create a new browser context and login as a specific user."""
    context = browser.new_context(base_url=BASE_URL)
    page = context.new_page()
    _do_login(page, email, password)
    return page


@pytest.fixture
def red_user_page(browser: Browser, create_test_user):
    """A page logged in as a Red role user."""
    creds = create_test_user(roles=["Red"])
    page = _login_as_user(browser, creds["email"], creds["password"])
    yield page
    page.context.close()


@pytest.fixture
def blue_user_page(browser: Browser, create_test_user):
    """A page logged in as a Blue role user."""
    creds = create_test_user(roles=["Blue"])
    page = _login_as_user(browser, creds["email"], creds["password"])
    yield page
    page.context.close()


@pytest.fixture
def spectator_page(browser: Browser, create_test_user):
    """A page logged in as a Spectator role user."""
    creds = create_test_user(roles=["Spectator"])
    page = _login_as_user(browser, creds["email"], creds["password"])
    yield page
    page.context.close()


@pytest.fixture
def test_assessment(authenticated_page: Page):
    """Create a test assessment and return its ID. Cleaned up after test."""
    uid = uuid.uuid4().hex[:8]
    name = f"Test Assessment {uid}"
    assessments = AssessmentsPage(authenticated_page)
    assessments.navigate()
    assessments.create_assessment(name, f"Description for {name}")

    # Extract assessment ID from the table link
    link = assessments.table.locator(f'a:has-text("{name}")').first
    href = link.get_attribute("href")
    assessment_id = href.split("/assessment/")[-1]

    yield {"id": assessment_id, "name": name}

    # Cleanup
    try:
        assessments.navigate()
        assessments.table.locator(f'a:has-text("{name}")').first.wait_for(timeout=2000)
        row_idx = 0
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if name in rows.nth(i).inner_text():
                row_idx = i
                break
        assessments.delete_assessment_by_row(row_idx)
    except Exception:
        pass


@pytest.fixture
def test_testcase(authenticated_page: Page, test_assessment: dict):
    """Create a test testcase in the test assessment. Returns testcase info."""
    uid = uuid.uuid4().hex[:8]
    tc_name = f"Test Testcase {uid}"
    detail = AssessmentDetailPage(authenticated_page)
    detail.navigate(test_assessment["id"])
    detail.create_testcase(tc_name)

    # Extract testcase ID
    link = detail.table.locator(f'a:has-text("{tc_name}")').first
    href = link.get_attribute("href")
    testcase_id = href.split("/testcase/")[-1]

    return {
        "id": testcase_id,
        "name": tc_name,
        "assessment_id": test_assessment["id"],
        "assessment_name": test_assessment["name"],
    }


@pytest.fixture
def temp_file():
    """Create a temporary file for upload tests. Returns path."""
    files = []

    def _create(content: str = "test content", suffix: str = ".txt"):
        f = tempfile.NamedTemporaryFile(delete=False, suffix=suffix, mode="w")
        f.write(content)
        f.close()
        files.append(f.name)
        return f.name

    yield _create

    for f in files:
        try:
            os.unlink(f)
        except Exception:
            pass
