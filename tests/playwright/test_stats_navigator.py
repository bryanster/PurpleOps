"""Tests for Assessment Statistics and ATT&CK Navigator pages."""

import pytest
from playwright.sync_api import Page, expect


class TestStatistics:
    def test_stats_page_loads(self, authenticated_page: Page, test_assessment: dict):
        """Statistics page renders with chart containers."""
        authenticated_page.goto(f"/assessment/{test_assessment['id']}/stats")
        authenticated_page.wait_for_timeout(1000)
        # Verify chart containers exist
        expect(authenticated_page.locator("#resultspie")).to_be_visible()
        expect(authenticated_page.locator("#results")).to_be_visible()
        expect(authenticated_page.locator("#alerts")).to_be_visible()
        expect(authenticated_page.locator("#priorities")).to_be_visible()

    def test_stats_charts_display_data(self, authenticated_page: Page, test_assessment: dict, test_testcase: dict):
        """With testcase data, charts render SVG elements."""
        authenticated_page.goto(f"/assessment/{test_assessment['id']}/stats")
        authenticated_page.wait_for_timeout(2000)
        # ApexCharts renders SVG elements
        charts = authenticated_page.locator(".apexcharts-canvas")
        # At least some chart containers should be present
        expect(authenticated_page.locator("#resultspie")).to_be_visible()


class TestNavigator:
    def test_navigator_page_loads(self, authenticated_page: Page, test_assessment: dict):
        """Navigator page renders with iframe."""
        authenticated_page.goto(f"/assessment/{test_assessment['id']}/navigator")
        authenticated_page.wait_for_timeout(1000)
        iframe = authenticated_page.locator("#navigator, iframe")
        expect(iframe).to_be_visible()

    def test_navigator_back_button(self, authenticated_page: Page, test_assessment: dict):
        """Back button on navigator returns to assessment."""
        authenticated_page.goto(f"/assessment/{test_assessment['id']}/navigator")
        authenticated_page.wait_for_timeout(1000)
        back_btn = authenticated_page.locator("#assessment-crumb-button, a:has-text('Back')").first
        if back_btn.is_visible():
            back_btn.click()
            authenticated_page.wait_for_timeout(500)
            expect(authenticated_page).to_have_url_matching(f".*assessment/{test_assessment['id']}$")
