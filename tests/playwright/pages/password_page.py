"""Page Object Model for the Password Change page."""

from playwright.sync_api import Page, expect


class PasswordPage:
    URL = "/password/change"

    def __init__(self, page: Page):
        self.page = page
        self.current_password = page.locator("#password")
        self.new_password = page.locator("#new_password")
        self.confirm_password = page.locator("#new_password_confirm")
        self.submit_button = page.locator("#submit")
        self.flash_messages = page.locator(".alert")

    def navigate(self):
        self.page.goto(self.URL)

    def change_password(self, current: str, new: str, confirm: str):
        self.current_password.fill(current)
        self.new_password.fill(new)
        self.confirm_password.fill(confirm)
        self.submit_button.click()

    def expect_flash_message(self, text: str):
        expect(self.flash_messages).to_contain_text(text)

    def expect_form_visible(self):
        expect(self.current_password).to_be_visible()
        expect(self.new_password).to_be_visible()
        expect(self.confirm_password).to_be_visible()
        expect(self.submit_button).to_be_visible()
