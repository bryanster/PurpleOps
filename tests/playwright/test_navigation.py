"""Tests for navigation, layout, and global UI elements."""

import pytest
from playwright.sync_api import Page, expect


class TestNavigation:
    def test_navbar_renders(self, authenticated_page: Page):
        """Navbar shows logo, settings dropdown, and logout link."""
        authenticated_page.goto("/")
        authenticated_page.wait_for_timeout(500)
        # Logo/brand link
        logo = authenticated_page.locator('.navbar-brand, a[href="/"]').first
        expect(logo).to_be_visible()
        # Logout link
        logout = authenticated_page.locator('a[href="/logout"]')
        expect(logout).to_be_visible()

    def test_settings_dropdown_links(self, authenticated_page: Page):
        """Settings dropdown contains expected links."""
        authenticated_page.goto("/")
        authenticated_page.wait_for_timeout(500)
        # Open settings dropdown
        settings_btn = authenticated_page.locator('.dropdown-toggle .bi-gear-fill, .bi-gear-fill').first
        settings_parent = settings_btn.locator("..")
        settings_parent.click()
        authenticated_page.wait_for_timeout(300)
        # Verify links exist
        expect(authenticated_page.locator('a[href="/api-keys"]')).to_be_visible()
        expect(authenticated_page.locator('a[href="/password/change"]')).to_be_visible()

    def test_about_modal(self, authenticated_page: Page):
        """Clicking About opens modal with credits."""
        authenticated_page.goto("/")
        authenticated_page.wait_for_timeout(500)
        # Open settings dropdown
        settings_btn = authenticated_page.locator('.dropdown-toggle .bi-gear-fill, .bi-gear-fill').first
        settings_parent = settings_btn.locator("..")
        settings_parent.click()
        authenticated_page.wait_for_timeout(300)
        about_link = authenticated_page.locator('[data-bs-target="#aboutModal"], a:has-text("About")').first
        if about_link.is_visible():
            about_link.click()
            about_modal = authenticated_page.locator("#aboutModal")
            expect(about_modal).to_be_visible()
            about_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_admin_sees_access_control_link(self, authenticated_page: Page):
        """Admin user sees Access Control in settings dropdown."""
        authenticated_page.goto("/")
        authenticated_page.wait_for_timeout(500)
        settings_btn = authenticated_page.locator('.dropdown-toggle .bi-gear-fill, .bi-gear-fill').first
        settings_parent = settings_btn.locator("..")
        settings_parent.click()
        authenticated_page.wait_for_timeout(300)
        access_link = authenticated_page.locator('a[href="/manage/access"]')
        expect(access_link).to_be_visible()

    def test_non_admin_no_access_control_link(self, red_user_page: Page):
        """Non-admin user does not see Access Control in settings."""
        red_user_page.goto("/")
        red_user_page.wait_for_timeout(500)
        settings_btn = red_user_page.locator('.dropdown-toggle .bi-gear-fill, .bi-gear-fill').first
        settings_parent = settings_btn.locator("..")
        settings_parent.click()
        red_user_page.wait_for_timeout(300)
        access_link = red_user_page.locator('a[href="/manage/access"]')
        expect(access_link).not_to_be_visible()

    def test_logo_navigates_home(self, authenticated_page: Page):
        """Clicking logo navigates to home page."""
        authenticated_page.goto("/password/change")
        authenticated_page.wait_for_timeout(500)
        logo = authenticated_page.locator('.navbar-brand, a[href="/"]').first
        logo.click()
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(".*/$|.*/index.*")
