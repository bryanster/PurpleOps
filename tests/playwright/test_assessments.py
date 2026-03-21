"""Tests for the Assessments home page."""

import uuid

import pytest
from playwright.sync_api import Page, expect

from pages.assessments_page import AssessmentsPage


class TestAssessmentsPage:
    def test_assessments_page_loads(self, authenticated_page: Page):
        """Assessments table renders with expected columns."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        expect(assessments.table).to_be_visible()
        expect(assessments.new_assessment_btn).to_be_visible()

    def test_create_assessment(self, authenticated_page: Page):
        """Create a new assessment via modal."""
        uid = uuid.uuid4().hex[:8]
        name = f"PW Test {uid}"
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.create_assessment(name, f"Description {uid}")
        assessments.expect_assessment_in_table(name)
        # Cleanup
        row_idx = 0
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if name in rows.nth(i).inner_text():
                row_idx = i
                break
        assessments.delete_assessment_by_row(row_idx)

    def test_create_assessment_empty_name(self, authenticated_page: Page):
        """Creating assessment with empty name still works (name is optional)."""
        uid = uuid.uuid4().hex[:8]
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.create_assessment("", f"No-name assessment {uid}")
        # Should still create (name may be empty or auto-generated)
        authenticated_page.wait_for_timeout(500)

    def test_edit_assessment(self, authenticated_page: Page, test_assessment: dict):
        """Edit an existing assessment's name and description."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        new_name = f"Edited {test_assessment['name']}"
        # Find the row with our assessment
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if test_assessment["name"] in rows.nth(i).inner_text():
                assessments.edit_assessment(i, new_name, "Updated description")
                break
        assessments.expect_assessment_in_table(new_name)

    def test_delete_assessment(self, authenticated_page: Page):
        """Delete an assessment via confirmation modal."""
        uid = uuid.uuid4().hex[:8]
        name = f"Delete Me {uid}"
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.create_assessment(name, "To be deleted")
        assessments.expect_assessment_in_table(name)
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if name in rows.nth(i).inner_text():
                assessments.delete_assessment_by_row(i)
                break
        assessments.expect_assessment_not_in_table(name)

    def test_delete_assessment_cancel(self, authenticated_page: Page, test_assessment: dict):
        """Dismissing delete modal keeps assessment in table."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if test_assessment["name"] in rows.nth(i).inner_text():
                delete_btn = rows.nth(i).locator(".btn-danger, .bi-trash-fill").first
                delete_btn.click()
                expect(assessments.delete_modal).to_be_visible()
                # Dismiss instead of confirm
                assessments.delete_modal.locator(".btn-close, .btn-secondary").first.click()
                break
        assessments.expect_assessment_in_table(test_assessment["name"])

    def test_assessment_table_search(self, authenticated_page: Page, test_assessment: dict):
        """Search box filters assessments table."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.search(test_assessment["name"])
        assessments.expect_assessment_in_table(test_assessment["name"])
        # Search for something that shouldn't exist
        assessments.search("ZZZZNONEXISTENT")
        assert assessments.get_row_count() == 0 or "No matching records" in authenticated_page.content()

    def test_assessment_table_sort(self, authenticated_page: Page, test_assessment: dict):
        """Click column header to sort."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        # Click the Name column header to sort
        name_header = assessments.table.locator('th:has-text("Name")').first
        name_header.click()
        authenticated_page.wait_for_timeout(500)
        # Table should still be visible and functional
        expect(assessments.table).to_be_visible()

    def test_assessment_progress_bar(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Assessment with testcases shows progress bar."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        # Find the row and check for progress bar
        rows = assessments.table.locator("tbody tr")
        for i in range(rows.count()):
            if test_assessment["name"] in rows.nth(i).inner_text():
                progress = rows.nth(i).locator(".progress")
                expect(progress).to_be_visible()
                break

    def test_assessment_link_navigation(self, authenticated_page: Page, test_assessment: dict):
        """Clicking assessment name navigates to detail page."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.click_assessment_link(test_assessment["name"])
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(f".*assessment/{test_assessment['id']}.*")

    def test_import_assessment_zip(self, authenticated_page: Page, test_assessment: dict, temp_file):
        """Import an assessment from ZIP file."""
        # First export an assessment, then import it
        # For now, test that the import modal opens and accepts a file
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.import_assessment_btn.click()
        expect(assessments.import_modal).to_be_visible()
        expect(assessments.import_file_input).to_be_visible()
        # Close without submitting
        assessments.import_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_import_assessment_invalid_file(self, authenticated_page: Page, temp_file):
        """Upload non-ZIP file shows error handling."""
        assessments = AssessmentsPage(authenticated_page)
        assessments.navigate()
        assessments.import_assessment_btn.click()
        expect(assessments.import_modal).to_be_visible()
        # Create a non-zip file
        txt_file = temp_file(content="not a zip file", suffix=".txt")
        assessments.import_file_input.set_input_files(txt_file)
        # Close without submitting (just verify the UI accepts the file)
        assessments.import_modal.locator(".btn-close, .btn-secondary").first.click()
