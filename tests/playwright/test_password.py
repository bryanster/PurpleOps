"""Tests for the Password Change page."""

import pytest
from playwright.sync_api import Page, expect

from pages.password_page import PasswordPage


ADMIN_PASSWORD = "TestAdmin123!"


class TestPasswordChange:
    def test_password_change_page_loads(self, authenticated_page: Page):
        """Password change form renders with all fields."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        pwd.expect_form_visible()

    def test_password_change_success(self, authenticated_page: Page):
        """Valid password change with matching passwords succeeds."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        new_password = "NewSecurePass123!"
        pwd.change_password(ADMIN_PASSWORD, new_password, new_password)
        authenticated_page.wait_for_timeout(1000)
        # Change it back to the original
        pwd.navigate()
        pwd.change_password(new_password, ADMIN_PASSWORD, ADMIN_PASSWORD)
        authenticated_page.wait_for_timeout(1000)

    def test_password_change_wrong_current(self, authenticated_page: Page):
        """Wrong current password shows error."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        pwd.change_password("wrongcurrentpassword", "NewPass123456!", "NewPass123456!")
        authenticated_page.wait_for_timeout(500)
        # Should remain on password change page with error
        expect(authenticated_page).to_have_url_matching(".*password.*")

    def test_password_change_mismatch(self, authenticated_page: Page):
        """Mismatched new passwords shows error."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        pwd.change_password(ADMIN_PASSWORD, "NewPass123456!", "DifferentPass123!")
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(".*password.*")

    def test_password_change_too_short(self, authenticated_page: Page):
        """New password shorter than 12 chars shows error."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        pwd.change_password(ADMIN_PASSWORD, "short", "short")
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(".*password.*")

    def test_password_change_redirect_after(self, authenticated_page: Page, browser):
        """After changing password, can login with new password."""
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        new_password = "ChangedPass12345!"
        pwd.change_password(ADMIN_PASSWORD, new_password, new_password)
        authenticated_page.wait_for_timeout(1000)
        # Logout and try logging in with new password
        authenticated_page.goto("/logout")
        authenticated_page.wait_for_timeout(500)
        from pages.login_page import LoginPage
        login = LoginPage(authenticated_page)
        login.login("admin@purpleops.com", new_password)
        authenticated_page.wait_for_timeout(1000)
        assert "/login" not in authenticated_page.url
        # Change back
        pwd = PasswordPage(authenticated_page)
        pwd.navigate()
        pwd.change_password(new_password, ADMIN_PASSWORD, ADMIN_PASSWORD)
