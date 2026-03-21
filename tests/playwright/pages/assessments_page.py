"""Page Object Model for the Assessments (home) page."""

from playwright.sync_api import Page, expect


class AssessmentsPage:
    URL = "/"

    def __init__(self, page: Page):
        self.page = page
        self.table = page.locator("#assessmentsTable")
        self.new_assessment_btn = page.locator("#newAssessment")
        self.import_assessment_btn = page.locator("#importAssessment")
        self.search_input = page.locator('.search-input input[type="search"]')

        # New/Edit Assessment modal
        self.modal = page.locator("#newAssessmentModal")
        self.name_input = page.locator("#newAssessmentModal #name")
        self.description_input = page.locator("#newAssessmentModal #description")
        self.modal_submit_btn = page.locator("#newAssessmentButton")

        # Import Assessment modal
        self.import_modal = page.locator("#importAssessmentModal")
        self.import_file_input = page.locator("#importAssessmentModal #formFile")
        self.import_submit_btn = page.locator(
            '#importAssessmentModal button[type="submit"]'
        )

        # Delete Assessment modal
        self.delete_modal = page.locator("#deleteAssessmentModal")
        self.delete_confirm_btn = page.locator("#deleteAssessmentButton")
        self.delete_warning = page.locator("#deleteAssessmentWarning")

    def navigate(self):
        self.page.goto(self.URL)
        self.page.wait_for_selector("#assessmentsTable")

    def create_assessment(self, name: str, description: str):
        self.new_assessment_btn.click()
        expect(self.modal).to_be_visible()
        self.name_input.fill(name)
        self.description_input.fill(description)
        with self.page.expect_response(lambda r: "/assessment" in r.url and r.status == 200):
            self.modal_submit_btn.click()
        self.page.wait_for_timeout(500)

    def edit_assessment(self, row_index: int, name: str, description: str):
        edit_btn = self.table.locator("tbody tr").nth(row_index).locator(".btn-warning, .bi-pencil-fill").first
        edit_btn.click()
        expect(self.modal).to_be_visible()
        self.name_input.fill(name)
        self.description_input.fill(description)
        with self.page.expect_response(lambda r: "/assessment" in r.url and r.status == 200):
            self.modal_submit_btn.click()
        self.page.wait_for_timeout(500)

    def delete_assessment_by_row(self, row_index: int):
        delete_btn = self.table.locator("tbody tr").nth(row_index).locator(".btn-danger, .bi-trash-fill").first
        delete_btn.click()
        expect(self.delete_modal).to_be_visible()
        with self.page.expect_response(lambda r: "/assessment" in r.url):
            self.delete_confirm_btn.click()
        self.page.wait_for_timeout(500)

    def import_assessment(self, file_path: str):
        self.import_assessment_btn.click()
        expect(self.import_modal).to_be_visible()
        self.import_file_input.set_input_files(file_path)
        with self.page.expect_response(lambda r: "import/entire" in r.url):
            self.import_submit_btn.click()
        self.page.wait_for_timeout(500)

    def get_row_count(self) -> int:
        return self.table.locator("tbody tr").count()

    def get_row_text(self, row_index: int) -> str:
        return self.table.locator("tbody tr").nth(row_index).inner_text()

    def search(self, query: str):
        self.search_input.fill(query)
        self.page.wait_for_timeout(500)

    def click_assessment_link(self, name: str):
        self.table.locator(f'a:has-text("{name}")').first.click()

    def expect_assessment_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_be_visible()

    def expect_assessment_not_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_have_count(0)

    def expect_progress_bar(self, row_index: int):
        progress = self.table.locator("tbody tr").nth(row_index).locator(".progress")
        expect(progress).to_be_visible()
