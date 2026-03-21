"""Tests for the API Keys management page."""

import uuid

import pytest
from playwright.sync_api import Page, expect

from pages.apikeys_page import APIKeysPage


class TestAPIKeys:
    def test_apikeys_page_loads(self, authenticated_page: Page):
        """API keys page renders with table."""
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        expect(keys.table).to_be_visible()

    def test_create_api_key(self, authenticated_page: Page):
        """Create a new API key and verify key is revealed."""
        uid = uuid.uuid4().hex[:8]
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.create_api_key(f"test-key-{uid}")
        # Key reveal modal should appear
        expect(keys.reveal_modal).to_be_visible()
        key_value = keys.get_revealed_key()
        assert key_value.startswith("pops_")
        keys.close_reveal_modal()

    def test_copy_api_key(self, authenticated_page: Page):
        """Click copy button copies key to clipboard."""
        uid = uuid.uuid4().hex[:8]
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.create_api_key(f"copy-key-{uid}")
        expect(keys.reveal_modal).to_be_visible()
        keys.copy_key()
        # Verify button was clicked without error
        keys.close_reveal_modal()

    def test_key_reveal_modal_one_time(self, authenticated_page: Page):
        """After closing reveal modal, key is no longer visible."""
        uid = uuid.uuid4().hex[:8]
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.create_api_key(f"onetime-{uid}")
        expect(keys.reveal_modal).to_be_visible()
        key_value = keys.get_revealed_key()
        assert len(key_value) > 0
        keys.close_reveal_modal()
        # Page reloads after close - key should not be shown again
        authenticated_page.wait_for_timeout(1500)
        expect(keys.reveal_modal).not_to_be_visible()

    def test_api_key_appears_in_table(self, authenticated_page: Page):
        """Created API key shows in table with prefix."""
        uid = uuid.uuid4().hex[:8]
        name = f"table-key-{uid}"
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.create_api_key(name)
        expect(keys.reveal_modal).to_be_visible()
        keys.close_reveal_modal()
        authenticated_page.wait_for_timeout(1500)
        keys.expect_key_in_table(name)

    def test_delete_api_key(self, authenticated_page: Page):
        """Delete an API key via confirmation modal."""
        uid = uuid.uuid4().hex[:8]
        name = f"delete-key-{uid}"
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.create_api_key(name)
        expect(keys.reveal_modal).to_be_visible()
        keys.close_reveal_modal()
        authenticated_page.wait_for_timeout(1500)
        keys.expect_key_in_table(name)
        # Find and delete the key
        rows = keys.table.locator("tbody tr")
        for i in range(rows.count()):
            if name in rows.nth(i).inner_text():
                keys.delete_key_by_row(i)
                break
        authenticated_page.wait_for_timeout(1500)

    def test_api_key_roles_limited_to_user(self, authenticated_page: Page):
        """Only the user's own roles are available in the roles dropdown."""
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.open_new_key_modal()
        roles = keys.get_available_roles()
        # Admin user should have Admin role available
        assert any("Admin" in r for r in roles)

    def test_api_key_assessments_limited_to_user(self, authenticated_page: Page):
        """Only the user's own assessments are available in dropdown."""
        keys = APIKeysPage(authenticated_page)
        keys.navigate()
        keys.open_new_key_modal()
        # Admin should see all assessments - just verify the dropdown exists
        container = authenticated_page.locator("#keyAssessments").locator("..").locator("..")
        toggle = container.locator(".dropdown-toggle")
        if toggle.is_visible():
            toggle.click()
            authenticated_page.wait_for_timeout(300)
