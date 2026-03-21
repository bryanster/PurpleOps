"""Page Object Model for the MFA Registration and Verification pages."""

from playwright.sync_api import Page, expect


class MFARegisterPage:
    URL = "/mfa/register"

    def __init__(self, page: Page):
        self.page = page
        self.setup_form = page.locator("#two_factor_setup_form, form[action='/mfa/register']")
        self.generate_btn = page.locator('input[type="submit"][value*="Generate"], #submit').first
        self.qrcode_img = page.locator("#qrcode")
        self.code_input = page.locator("#code")
        self.verify_btn = page.locator('input[value="Submit Code"]')
        self.flash_messages = page.locator(".alert")

    def navigate(self):
        self.page.goto(self.URL)

    def generate_qr_code(self):
        self.generate_btn.click()

    def enter_code(self, code: str):
        self.code_input.fill(code)
        self.verify_btn.click()

    def expect_qr_visible(self):
        expect(self.qrcode_img).to_be_visible()


class MFAVerifyPage:
    URL = "/mfa/verify"

    def __init__(self, page: Page):
        self.page = page
        self.code_input = page.locator("#code")
        self.submit_btn = page.locator('input[value="Submit Code"], #submit')
        self.flash_messages = page.locator(".alert")

    def navigate(self):
        self.page.goto(self.URL)

    def verify(self, code: str):
        self.code_input.fill(code)
        self.submit_btn.click()

    def expect_form_visible(self):
        expect(self.code_input).to_be_visible()
        expect(self.submit_btn).to_be_visible()
