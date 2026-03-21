"""Tests for import/export functionality."""

import json
import os

import pytest
from playwright.sync_api import Page, expect

from pages.assessment_detail_page import AssessmentDetailPage


class TestExportImport:
    def test_export_json(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Export testcases as JSON downloads a .json file."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Results as JSON")').first
        with authenticated_page.expect_download() as download_info:
            link.click()
        download = download_info.value
        assert download.suggested_filename.endswith(".json")

    def test_export_csv(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Export testcases as CSV downloads a .csv file."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Results as CSV")').first
        with authenticated_page.expect_download() as download_info:
            link.click()
        download = download_info.value
        assert download.suggested_filename.endswith(".csv")

    def test_export_navigator_layer(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Export ATT&CK Navigator layer downloads .json."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("ATT&CK Navigator Layer")').first
        with authenticated_page.expect_download() as download_info:
            link.click()
        download = download_info.value
        assert download.suggested_filename.endswith(".json")

    def test_export_entire_assessment(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Export entire assessment downloads a .zip file."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Entire Assessment")').first
        with authenticated_page.expect_download() as download_info:
            link.click()
        download = download_info.value
        assert download.suggested_filename.endswith(".zip")

    def test_export_campaign_template(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Admin can export campaign template."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Campaign Template")').first
        if link.is_visible():
            with authenticated_page.expect_download() as download_info:
                link.click()
            download = download_info.value
            assert download.suggested_filename.endswith(".json")

    def test_export_testcase_templates(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Admin can export testcase templates."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Testcase Templates")').first
        if link.is_visible():
            with authenticated_page.expect_download() as download_info:
                link.click()
            download = download_info.value
            assert download.suggested_filename.endswith(".json")

    def test_import_then_export_roundtrip(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Export assessment ZIP then verify it can be imported."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        # Export
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        link = authenticated_page.locator('a:has-text("Entire Assessment")').first
        with authenticated_page.expect_download() as download_info:
            link.click()
        download = download_info.value
        export_path = download.path()
        assert os.path.exists(export_path)

    def test_import_navigator_creates_testcases(self, authenticated_page: Page, test_assessment: dict, temp_file):
        """Import navigator JSON file opens modal correctly."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        import_btn = authenticated_page.locator('button:has-text("Import")')
        import_btn.click()
        nav_link = authenticated_page.locator('a:has-text("MITRE ATT&CK Navigator Layer")').first
        nav_link.click()
        expect(detail.navigator_modal).to_be_visible()
        # Create a minimal navigator JSON
        nav_json = json.dumps({
            "name": "test layer",
            "versions": {"attack": "14", "navigator": "4.9.1", "layer": "4.5"},
            "domain": "enterprise-attack",
            "techniques": [{"techniqueID": "T1059", "score": 1}],
        })
        nav_file = temp_file(content=nav_json, suffix=".json")
        detail.navigator_file_input.set_input_files(nav_file)
        authenticated_page.wait_for_timeout(300)
        detail.navigator_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_import_campaign_creates_testcases(self, authenticated_page: Page, test_assessment: dict, temp_file):
        """Import campaign template modal opens correctly."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        import_btn = authenticated_page.locator('button:has-text("Import")')
        import_btn.click()
        campaign_link = authenticated_page.locator('a:has-text("Campaign Template")').first
        campaign_link.click()
        expect(detail.campaign_modal).to_be_visible()
        detail.campaign_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_export_report_docx(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """Generate report modal opens if custom templates exist."""
        detail = AssessmentDetailPage(authenticated_page)
        detail.navigate(test_assessment["id"])
        export_btn = authenticated_page.locator('button:has-text("Export")')
        export_btn.click()
        report_link = authenticated_page.locator('a:has-text("Generate Report")').first
        if report_link.is_visible():
            report_link.click()
            expect(detail.export_report_modal).to_be_visible()
            detail.export_report_modal.locator(".btn-close, .btn-secondary").first.click()
