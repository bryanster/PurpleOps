"""Page Object Model for the Access Control (user management) page."""

from playwright.sync_api import Page, expect


class AccessPage:
    URL = "/manage/access"

    def __init__(self, page: Page):
        self.page = page
        self.table = page.locator("#userTable")
        self.add_user_btn = page.locator("#add")

        # User detail modal
        self.detail_modal = page.locator("#userDetailModal")
        self.detail_form = page.locator("#userDetailForm")
        self.username_input = page.locator("#userDetailModal #username")
        self.email_input = page.locator("#userDetailModal #email")
        self.password_input = page.locator("#userDetailModal #password")
        self.roles_select = page.locator("#userDetailModal #roles")
        self.assessments_select = page.locator("#userDetailModal #assessments")
        self.detail_submit_btn = page.locator("#userDetailButton")

        # Delete user modal
        self.delete_modal = page.locator("#deleteUserModal")
        self.delete_confirm_btn = page.locator("#deleteUserButton")
        self.delete_warning = page.locator("#deleteUserWarning")

        # Random password button
        self.random_password_btn = page.locator('button:has-text("Random")')

    def navigate(self):
        self.page.goto(self.URL)
        self.page.wait_for_selector("#userTable")

    def create_user(
        self,
        email: str,
        username: str,
        password: str,
        roles: list[str] = None,
        assessments: list[str] = None,
    ):
        self.add_user_btn.click()
        expect(self.detail_modal).to_be_visible()
        self.email_input.fill(email)
        self.username_input.fill(username)
        self.password_input.fill(password)
        if roles:
            self._select_multiple("#userDetailModal #roles", roles)
        if assessments:
            self._select_multiple("#userDetailModal #assessments", assessments)
        with self.page.expect_response(lambda r: "/manage/access/user" in r.url and r.status == 200):
            self.detail_submit_btn.click()
        self.page.wait_for_timeout(500)

    def _select_multiple(self, selector: str, values: list[str]):
        container = self.page.locator(selector).locator("..").locator("..")
        container.locator(".dropdown-toggle").click()
        for val in values:
            container.locator(f'.dropdown-item:has-text("{val}")').click()
        # Close dropdown by clicking outside
        self.page.locator(".modal-header").first.click()

    def edit_user_by_row(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        edit_btn = row.locator(".btn-warning, .bi-pencil-fill").first
        edit_btn.click()
        expect(self.detail_modal).to_be_visible()

    def delete_user_by_row(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        delete_btn = row.locator(".btn-danger, .bi-trash-fill").first
        delete_btn.click()
        expect(self.delete_modal).to_be_visible()
        with self.page.expect_response(lambda r: "/manage/access/user" in r.url):
            self.delete_confirm_btn.click()
        self.page.wait_for_timeout(500)

    def click_random_password(self):
        self.random_password_btn.click()

    def get_user_count(self) -> int:
        return self.table.locator("tbody tr").count()

    def get_row_text(self, row_index: int) -> str:
        return self.table.locator("tbody tr").nth(row_index).inner_text()

    def expect_user_in_table(self, username: str):
        expect(self.table.locator(f'td:has-text("{username}")')).to_be_visible()

    def expect_user_not_in_table(self, username: str):
        expect(self.table.locator(f'td:has-text("{username}")')).to_have_count(0)

    def find_user_row(self, username: str):
        rows = self.table.locator("tbody tr")
        for i in range(rows.count()):
            if username in rows.nth(i).inner_text():
                return i
        return -1
