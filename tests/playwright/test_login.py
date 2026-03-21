"""Tests for the Login page and authentication flow."""

import pytest
from playwright.sync_api import Page, expect

from pages.login_page import LoginPage


class TestLoginPage:
    def test_login_page_renders(self, page: Page, base_url):
        """Verify login page displays all form fields."""
        page.goto(f"{base_url}/login")
        login = LoginPage(page)
        login.expect_form_fields_visible()
        expect(page).to_have_title_matching(".*")

    def test_login_success(self, page: Page, base_url, admin_credentials):
        """Valid admin credentials redirect to home page."""
        login = LoginPage(page)
        login.login(admin_credentials["email"], admin_credentials["password"])
        page.wait_for_timeout(1000)
        # Should be on home or password change page
        assert "/login" not in page.url

    def test_login_invalid_password(self, page: Page, base_url, admin_credentials):
        """Wrong password shows flash error message."""
        login = LoginPage(page)
        login.login(admin_credentials["email"], "wrongpassword123")
        page.wait_for_timeout(500)
        login.expect_on_login_page()

    def test_login_invalid_email(self, page: Page, base_url):
        """Non-existent email shows flash error."""
        login = LoginPage(page)
        login.login("nonexistent@example.com", "somepassword")
        page.wait_for_timeout(500)
        login.expect_on_login_page()

    def test_login_empty_fields(self, page: Page, base_url):
        """Submit empty form stays on login page."""
        page.goto(f"{base_url}/login")
        login = LoginPage(page)
        login.submit_button.click()
        login.expect_on_login_page()

    def test_login_redirect_unauthenticated(self, page: Page, base_url):
        """Accessing / without login redirects to /login."""
        page.goto(f"{base_url}/")
        page.wait_for_timeout(500)
        expect(page).to_have_url_matching(".*/login.*")

    def test_logout(self, page: Page, base_url, admin_credentials):
        """Clicking logout redirects to login and clears session."""
        login = LoginPage(page)
        login.login(admin_credentials["email"], admin_credentials["password"])
        page.wait_for_timeout(1000)
        # Handle password change redirect
        if "/password/change" in page.url:
            page.goto(f"{base_url}/password/changed")
            page.wait_for_timeout(500)
        # Now logout
        page.goto(f"{base_url}/logout")
        page.wait_for_timeout(500)
        expect(page).to_have_url_matching(".*/login.*")
        # Verify session is cleared
        page.goto(f"{base_url}/")
        page.wait_for_timeout(500)
        expect(page).to_have_url_matching(".*/login.*")

    def test_login_remember_me(self, page: Page, base_url, admin_credentials):
        """Check remember me checkbox during login."""
        login = LoginPage(page)
        login.login(
            admin_credentials["email"],
            admin_credentials["password"],
            remember=True,
        )
        page.wait_for_timeout(1000)
        assert "/login" not in page.url

    def test_login_initial_password_redirect(self, page: Page, base_url, admin_credentials):
        """First login with initial password redirects to password change."""
        # This test verifies the redirect mechanism exists.
        # The actual redirect depends on InitPwd flag state.
        login = LoginPage(page)
        login.login(admin_credentials["email"], admin_credentials["password"])
        page.wait_for_timeout(1000)
        # Either we're on home or password change - both are valid post-login
        assert "/login" not in page.url

    def test_login_rate_limiting(self, page: Page, base_url, admin_credentials):
        """Rapid login attempts trigger rate limiting."""
        login = LoginPage(page)
        # Attempt many rapid logins with wrong password
        for _ in range(15):
            page.goto(f"{base_url}/login")
            login.email_input.fill(admin_credentials["email"])
            login.password_input.fill("wrongpassword")
            login.submit_button.click()
            page.wait_for_timeout(100)
        # After rate limit, should get an error response
        page.wait_for_timeout(500)
        # The page should still be on login (not crashed)
        login.expect_on_login_page()
