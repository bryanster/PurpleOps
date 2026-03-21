"""Page Object Model for the Login page."""

from playwright.sync_api import Page, expect


class LoginPage:
    URL = "/login"

    def __init__(self, page: Page):
        self.page = page
        self.email_input = page.locator("#email")
        self.password_input = page.locator("#password")
        self.remember_checkbox = page.locator("#remember")
        self.submit_button = page.locator("#submit")
        self.flash_messages = page.locator(".alert")
        self.oauth_button = page.locator('a[href="/auth/oauth/login"]')
        self.saml_button = page.locator('a[href="/auth/saml/login"]')

    def navigate(self):
        self.page.goto(self.URL)

    def login(self, email: str, password: str, remember: bool = False):
        self.navigate()
        self.email_input.fill(email)
        self.password_input.fill(password)
        if remember:
            self.remember_checkbox.check()
        self.submit_button.click()

    def expect_flash_message(self, text: str):
        expect(self.flash_messages).to_contain_text(text)

    def expect_on_login_page(self):
        expect(self.page).to_have_url_matching(self.URL)
        expect(self.email_input).to_be_visible()

    def expect_form_fields_visible(self):
        expect(self.email_input).to_be_visible()
        expect(self.password_input).to_be_visible()
        expect(self.submit_button).to_be_visible()
