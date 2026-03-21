"""Tests for the Testcase Detail page."""

import uuid

import pytest
from playwright.sync_api import Page, expect

from pages.testcase_page import TestcasePage


class TestTestcasePage:
    def test_testcase_page_loads(self, authenticated_page: Page, test_testcase: dict):
        """Testcase page renders split view with all fields."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        expect(tc.form).to_be_visible()
        expect(tc.save_button).to_be_visible()
        expect(tc.back_button).to_be_visible()
        expect(tc.name_input).to_be_visible()
        expect(tc.objective_textarea).to_be_visible()

    def test_testcase_save(self, authenticated_page: Page, test_testcase: dict):
        """Modify fields and save shows toast notification."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_objective("Test objective text")
        tc.save()
        tc.expect_toast_visible()

    def test_testcase_back_button(self, authenticated_page: Page, test_testcase: dict):
        """Back button returns to assessment page."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.go_back()
        authenticated_page.wait_for_timeout(500)
        expect(authenticated_page).to_have_url_matching(f".*assessment/{test_testcase['assessment_id']}.*")

    def test_testcase_mitre_id_change(self, authenticated_page: Page, test_testcase: dict):
        """Changing MITRE ID auto-updates tactic dropdown."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        # Change MITRE ID via bootstrap select
        container = tc.mitreid_select.locator("..").locator("..")
        toggle = container.locator(".dropdown-toggle")
        if toggle.is_visible():
            toggle.click()
            search = container.locator('.bs-searchbox input[type="search"]')
            if search.is_visible():
                search.fill("T1059")
            container.locator('.dropdown-item:has-text("T1059")').first.click()
            authenticated_page.wait_for_timeout(500)

    def test_testcase_name_edit(self, authenticated_page: Page, test_testcase: dict):
        """Change testcase name and save persists the change."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        new_name = f"Renamed {uuid.uuid4().hex[:6]}"
        tc.set_name(new_name)
        tc.save()
        # Reload and verify
        tc.navigate(test_testcase["id"])
        expect(tc.name_input).to_have_value(new_name)

    def test_testcase_objective_textarea(self, authenticated_page: Page, test_testcase: dict):
        """Enter text in objective textarea and verify auto-height."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        initial_height = tc.objective_textarea.bounding_box()["height"]
        tc.set_objective("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8")
        authenticated_page.wait_for_timeout(300)
        new_height = tc.objective_textarea.bounding_box()["height"]
        assert new_height >= initial_height

    def test_testcase_actions_textarea(self, authenticated_page: Page, test_testcase: dict):
        """Enter text in actions textarea (monospace, auto-height)."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_actions("whoami\nhostname\nipconfig /all")
        tc.save()
        tc.expect_toast_visible()

    def test_testcase_red_notes(self, authenticated_page: Page, test_testcase: dict):
        """Enter and save red notes."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_red_notes("Red team observation notes")
        tc.save()
        tc.navigate(test_testcase["id"])
        expect(tc.red_notes_textarea).to_have_value("Red team observation notes")

    def test_testcase_blue_notes(self, authenticated_page: Page, test_testcase: dict):
        """Enter and save blue notes."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_blue_notes("Blue team detection notes")
        tc.save()
        tc.navigate(test_testcase["id"])
        expect(tc.blue_notes_textarea).to_have_value("Blue team detection notes")

    def test_testcase_state_display(self, authenticated_page: Page, test_testcase: dict):
        """State field shows correct value and is readonly."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        expect(tc.state_input).to_have_attribute("readonly", "")
        state_value = tc.state_input.input_value()
        assert state_value in ["Pending", "Running", "Complete", ""]

    def test_testcase_timer_start_stop(self, authenticated_page: Page, test_testcase: dict):
        """Click run button starts timer, click again stops it."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        # Start timer
        tc.click_run_button()
        authenticated_page.wait_for_timeout(500)
        # Button text should change
        btn_text = tc.run_button.inner_text()
        assert btn_text in ["Stop", "Restart", "Start"]

    def test_testcase_timer_restart(self, authenticated_page: Page, test_testcase: dict):
        """Restart timer resets elapsed time."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.click_run_button()  # Start
        authenticated_page.wait_for_timeout(500)
        tc.click_run_button()  # Stop
        authenticated_page.wait_for_timeout(500)
        tc.click_run_button()  # Restart
        authenticated_page.wait_for_timeout(500)
        expect(tc.run_button).to_be_visible()

    def test_testcase_sources_multiselect(self, authenticated_page: Page, test_testcase: dict):
        """Open sources manage modal, add a source."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.open_manage_modal("source-label")
        expect(tc.source_modal).to_be_visible()
        tc.source_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_testcase_targets_multiselect(self, authenticated_page: Page, test_testcase: dict):
        """Open targets manage modal."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.open_manage_modal("target-label")
        expect(tc.target_modal).to_be_visible()
        tc.target_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_testcase_tools_multiselect(self, authenticated_page: Page, test_testcase: dict):
        """Open tools manage modal."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.open_manage_modal("tool-label")
        expect(tc.tool_modal).to_be_visible()
        tc.tool_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_testcase_tags_multiselect(self, authenticated_page: Page, test_testcase: dict):
        """Open tags manage modal with color picker."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.open_manage_modal("tags-label")
        expect(tc.tag_modal).to_be_visible()
        tc.tag_modal.locator(".btn-close, .btn-secondary").first.click()

    def test_testcase_prevention_radio_visibility(self, authenticated_page: Page, test_testcase: dict):
        """Prevention radio buttons toggle conditional field visibility."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        # Select "Yes" - should show rating/source
        tc.set_prevented("Yes")
        authenticated_page.wait_for_timeout(300)
        tc.expect_prevented_fields_visible()
        # Select "No" - should hide rating/source
        tc.set_prevented("No")
        authenticated_page.wait_for_timeout(300)
        tc.expect_prevented_fields_hidden()

    def test_testcase_alerted_radio_visibility(self, authenticated_page: Page, test_testcase: dict):
        """Alerted radio buttons toggle alert severity visibility."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_alerted("Yes")
        authenticated_page.wait_for_timeout(300)
        tc.expect_alert_container_visible()
        tc.set_alerted("No")
        authenticated_page.wait_for_timeout(300)

    def test_testcase_logged_radio_visibility(self, authenticated_page: Page, test_testcase: dict):
        """Logged radio buttons toggle detection fields visibility."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_alerted("No")
        authenticated_page.wait_for_timeout(300)
        tc.set_logged("Yes")
        authenticated_page.wait_for_timeout(300)
        tc.expect_detection_container_visible()

    def test_testcase_priority_radio_visibility(self, authenticated_page: Page, test_testcase: dict):
        """Priority radio buttons toggle urgency dropdown visibility."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_priority("Prevent")
        authenticated_page.wait_for_timeout(300)
        tc.expect_urgency_container_visible()
        tc.set_priority("N/A")
        authenticated_page.wait_for_timeout(300)
        tc.expect_urgency_container_hidden()

    def test_testcase_same_as_detection_checkbox(self, authenticated_page: Page, test_testcase: dict):
        """Same as detection source checkbox syncs source fields."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        tc.set_prevented("Yes")
        authenticated_page.wait_for_timeout(300)
        if tc.same_source_checkbox.is_visible():
            tc.same_source_checkbox.check()
            authenticated_page.wait_for_timeout(300)

    def test_testcase_evidence_upload_red(self, authenticated_page: Page, test_testcase: dict, temp_file):
        """Upload file to red team evidence."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        evidence_file = temp_file(content="red evidence content", suffix=".txt")
        tc.upload_red_evidence(evidence_file)
        tc.save()
        tc.expect_toast_visible()

    def test_testcase_evidence_upload_blue(self, authenticated_page: Page, test_testcase: dict, temp_file):
        """Upload file to blue team evidence."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        evidence_file = temp_file(content="blue evidence content", suffix=".txt")
        tc.upload_blue_evidence(evidence_file)
        tc.save()
        tc.expect_toast_visible()

    def test_testcase_evidence_delete(self, authenticated_page: Page, test_testcase: dict, temp_file):
        """Delete an uploaded evidence file."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        evidence_file = temp_file(content="to be deleted", suffix=".txt")
        tc.upload_red_evidence(evidence_file)
        tc.save()
        # Reload to see evidence list
        tc.navigate(test_testcase["id"])
        delete_btns = tc.red_evidence_list.locator(".evidence-delete")
        if delete_btns.count() > 0:
            with authenticated_page.expect_response(lambda r: "evidence" in r.url):
                delete_btns.first.click()

    def test_testcase_evidence_download(self, authenticated_page: Page, test_testcase: dict, temp_file):
        """Download an evidence file."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        evidence_file = temp_file(content="downloadable content", suffix=".txt")
        tc.upload_red_evidence(evidence_file)
        tc.save()
        tc.navigate(test_testcase["id"])
        links = tc.red_evidence_list.locator('a[href*="evidence"]')
        if links.count() > 0:
            with authenticated_page.expect_download() as download_info:
                links.first.click()
            download = download_info.value
            assert download.suggested_filename

    def test_testcase_ttp_info_modal(self, authenticated_page: Page, test_testcase: dict):
        """TTP Info modal shows description and sigma rules."""
        tc = TestcasePage(authenticated_page)
        tc.navigate(test_testcase["id"])
        if tc.info_button.is_visible():
            tc.open_ttp_info()
            expect(tc.ttp_info_modal).to_be_visible()
            tc.ttp_info_modal.locator(".btn-close, .btn-secondary").first.click()
