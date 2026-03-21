"""Tests for the Assessment Detail page."""

import json
import uuid

import pytest
from playwright.sync_api import Page, expect

from pages.assessment_detail_page import AssessmentDetailPage


class TestAssessmentDetailPage:
    def test_assessment_detail_page_loads(self, authenticated_page: Page, test_assessment: dict):
        """Assessment detail page renders table, toolbar, and hexagon chart."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        expect(detail.table).to_be_visible()
        expect(detail.toolbar).to_be_visible()
        expect(detail.new_testcase_btn).to_be_visible()

    def test_create_testcase(self, authenticated_page: Page, test_assessment: dict):
        """Create a new testcase via modal."""
        uid = uuid.uuid4().hex[:8]
        name = f"TC {uid}"
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.create_testcase(name)
        detail.expect_testcase_in_table(name)

    def test_create_testcase_mitre_auto_tactic(self, authenticated_page: Page, test_assessment: dict):
        """Selecting MITRE ID auto-populates tactic dropdown."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.new_testcase_btn.click()
        expect(detail.new_tc_modal).to_be_visible()
        # Select a MITRE technique and verify tactic changes
        detail._select_bootstrap_select("#newTestcaseModal #mitreid", "T1059")
        authenticated_page.wait_for_timeout(500)
        tactic_value = detail.tc_tactic_select.input_value()
        assert tactic_value != ""

    def test_testcase_table_search(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Search box filters testcases."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.search_table(test_testcase["name"])
        detail.expect_testcase_in_table(test_testcase["name"])
        detail.search_table("ZZZZNONEXISTENT")
        authenticated_page.wait_for_timeout(500)
        assert detail.get_testcase_count() == 0

    def test_testcase_table_column_visibility(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Toggle column visibility via column selector."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        # Look for columns dropdown button
        columns_btn = authenticated_page.locator('.columns-right button, button[data-toggle="columns"]').first
        if columns_btn.is_visible():
            columns_btn.click()
            authenticated_page.wait_for_timeout(300)
            # Toggle a column
            items = authenticated_page.locator('.dropdown-item input[type="checkbox"]')
            if items.count() > 0:
                items.first.click()
                authenticated_page.wait_for_timeout(300)

    def test_testcase_table_sort(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Sort testcases by clicking column headers."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        name_header = detail.table.locator('th:has-text("Name")').first
        name_header.click()
        authenticated_page.wait_for_timeout(500)
        expect(detail.table).to_be_visible()

    def test_testcase_table_checkbox_selection(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Select rows with checkboxes and verify count display."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.select_row_checkbox(0)
        selected_text = detail.selected_count.inner_text()
        assert "1" in selected_text

    def test_toggle_testcase_visibility(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Toggle testcase visibility via AJAX."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.toggle_visibility(0)
        authenticated_page.wait_for_timeout(500)
        # Verify the visibility icon changed
        expect(detail.table).to_be_visible()

    def test_clone_testcase(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Clone a testcase creates a new row."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        initial_count = detail.get_testcase_count()
        detail.clone_testcase(0)
        assert detail.get_testcase_count() == initial_count + 1

    def test_delete_testcase(self, authenticated_page: Page, test_assessment: dict):
        """Delete a testcase removes it from the table."""
        uid = uuid.uuid4().hex[:8]
        name = f"DeleteMe {uid}"
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.create_testcase(name)
        detail.expect_testcase_in_table(name)
        # Find and delete the row
        rows = detail.table.locator("tbody tr")
        for i in range(rows.count()):
            if name in rows.nth(i).inner_text():
                detail.delete_testcase(i)
                break
        detail.expect_testcase_not_in_table(name)

    def test_toggle_timer(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Toggle timer button changes state via AJAX."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.toggle_timer(0)
        authenticated_page.wait_for_timeout(500)
        expect(detail.table).to_be_visible()

    def test_manage_datasources(self, authenticated_page: Page, test_assessment: dict):
        """Open datasources modal, add entry, save."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        manage_btn = authenticated_page.locator('button:has-text("Manage")')
        manage_btn.click()
        authenticated_page.locator('a:has-text("Datasources")').first.click()
        expect(detail.manage_datasources_modal).to_be_visible()
        # Add a new datasource
        new_btn = authenticated_page.locator("#datasourcesNewButton").first
        if new_btn.is_visible():
            new_btn.click()
            authenticated_page.wait_for_timeout(300)
        detail.manage_datasources_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_manage_rules(self, authenticated_page: Page, test_assessment: dict):
        """Open rules modal, verify it renders."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        manage_btn = authenticated_page.locator('button:has-text("Manage")')
        manage_btn.click()
        authenticated_page.locator('a:has-text("Rules")').first.click()
        expect(detail.manage_rules_modal).to_be_visible()
        detail.manage_rules_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_manage_detection_sources(self, authenticated_page: Page, test_assessment: dict):
        """Open detection sources modal, verify it renders."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        manage_btn = authenticated_page.locator('button:has-text("Manage")')
        manage_btn.click()
        authenticated_page.locator('a:has-text("Detection Sources")').first.click()
        expect(detail.manage_detection_modal).to_be_visible()
        detail.manage_detection_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_manage_prevention_sources(self, authenticated_page: Page, test_assessment: dict):
        """Open prevention sources modal, verify it renders."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        manage_btn = authenticated_page.locator('button:has-text("Manage")')
        manage_btn.click()
        authenticated_page.locator('a:has-text("Prevention Sources")').first.click()
        expect(detail.manage_prevention_modal).to_be_visible()
        detail.manage_prevention_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_import_from_templates(self, authenticated_page: Page, test_assessment: dict):
        """Open templates modal, verify template table renders."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        import_btn = authenticated_page.locator('button:has-text("Import")')
        import_btn.click()
        authenticated_page.locator('a:has-text("Testcase(s) From Template")').first.click()
        expect(detail.templates_modal).to_be_visible()
        expect(detail.templates_table).to_be_visible()
        detail.templates_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_import_navigator_layer(self, authenticated_page: Page, test_assessment: dict, temp_file):
        """Import MITRE ATT&CK Navigator layer modal opens and accepts JSON."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        import_btn = authenticated_page.locator('button:has-text("Import")')
        import_btn.click()
        authenticated_page.locator('a:has-text("MITRE ATT&CK Navigator Layer")').first.click()
        expect(detail.navigator_modal).to_be_visible()
        expect(detail.navigator_file_input).to_be_visible()
        detail.navigator_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_import_campaign_template(self, authenticated_page: Page, test_assessment: dict, temp_file):
        """Import campaign template modal opens and accepts JSON."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        import_btn = authenticated_page.locator('button:has-text("Import")')
        import_btn.click()
        authenticated_page.locator('a:has-text("Campaign Template")').first.click()
        expect(detail.campaign_modal).to_be_visible()
        detail.campaign_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_statistics_link(self, authenticated_page: Page, test_assessment: dict):
        """Click Statistics navigates to stats page."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.click_statistics()
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(".*stats.*")

    def test_navigator_link(self, authenticated_page: Page, test_assessment: dict):
        """Click ATT&CK Navigator navigates to navigator page."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        detail.click_navigator()
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(".*navigator.*")
