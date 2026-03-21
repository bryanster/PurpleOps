"""Page Object Model for the API Keys management page."""

from playwright.sync_api import Page, expect


class APIKeysPage:
    URL = "/api-keys"

    def __init__(self, page: Page):
        self.page = page
        self.table = page.locator("#keyTable")

        # New API Key modal
        self.detail_modal = page.locator("#apiKeyDetailModal")
        self.key_name_input = page.locator("#keyName")
        self.key_roles_select = page.locator("#keyRoles")
        self.key_assessments_select = page.locator("#keyAssessments")
        self.create_btn = page.locator("#apiKeyDetailButton")

        # Key reveal modal
        self.reveal_modal = page.locator("#keyRevealModal")
        self.revealed_key = page.locator("#revealedKey")
        self.copy_btn = page.locator('button:has-text("Copy")')

        # Delete key modal
        self.delete_modal = page.locator("#deleteKeyModal")
        self.delete_confirm_btn = page.locator("#deleteKeyButton")
        self.delete_warning = page.locator("#deleteKeyWarning")

    def navigate(self):
        self.page.goto(self.URL)
        self.page.wait_for_selector("#keyTable")

    def open_new_key_modal(self):
        self.page.locator('button:has-text("New")').first.click()
        expect(self.detail_modal).to_be_visible()

    def create_api_key(self, name: str, roles: list[str] = None, assessments: list[str] = None):
        self.open_new_key_modal()
        self.key_name_input.fill(name)
        if roles:
            self._select_multiple("#keyRoles", roles)
        if assessments:
            self._select_multiple("#keyAssessments", assessments)
        with self.page.expect_response(lambda r: "/api-keys" in r.url and r.status == 200):
            self.create_btn.click()
        self.page.wait_for_timeout(500)

    def _select_multiple(self, selector: str, values: list[str]):
        container = self.page.locator(selector).locator("..").locator("..")
        container.locator(".dropdown-toggle").click()
        for val in values:
            container.locator(f'.dropdown-item:has-text("{val}")').click()
        self.page.locator(".modal-header").first.click()

    def get_revealed_key(self) -> str:
        expect(self.reveal_modal).to_be_visible()
        return self.revealed_key.input_value()

    def close_reveal_modal(self):
        self.reveal_modal.locator('button:has-text("Close"), .btn-close').first.click()

    def copy_key(self):
        self.copy_btn.click()

    def delete_key_by_row(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        delete_btn = row.locator(".btn-danger, [onclick*='confirmDeleteKey']").first
        delete_btn.click()
        expect(self.delete_modal).to_be_visible()
        with self.page.expect_response(lambda r: "/api-keys" in r.url):
            self.delete_confirm_btn.click()
        self.page.wait_for_timeout(1000)

    def get_key_count(self) -> int:
        return self.table.locator("tbody tr").count()

    def expect_key_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_be_visible()

    def expect_key_not_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_have_count(0)

    def get_available_roles(self) -> list[str]:
        container = self.page.locator("#keyRoles").locator("..").locator("..")
        container.locator(".dropdown-toggle").click()
        options = container.locator(".dropdown-item")
        values = [options.nth(i).inner_text() for i in range(options.count())]
        self.page.locator(".modal-header").first.click()
        return values
