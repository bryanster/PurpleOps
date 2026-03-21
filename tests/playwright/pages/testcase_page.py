"""Page Object Model for the Testcase Detail page."""

from playwright.sync_api import Page, expect


class TestcasePage:

    def __init__(self, page: Page):
        self.page = page
        self.form = page.locator("#ttpform")
        self.back_button = page.locator("#assessment-crumb-button")
        self.save_button = page.locator("#save")
        self.toast = page.locator("#toast")

        # Header fields
        self.name_input = page.locator("#name")
        self.mitreid_select = page.locator("#ttpform #mitreid")
        self.tactic_select = page.locator("#tactic")
        self.state_input = page.locator("#state")

        # Red team fields
        self.objective_textarea = page.locator("#objective")
        self.actions_textarea = page.locator("#actions")
        self.red_notes_textarea = page.locator("#rednotes")
        self.uuid_input = page.locator("#uuid")
        self.visible_checkbox = page.locator("#visible")

        # Timer
        self.run_button = page.locator("#run-button")
        self.elapsed_timer = page.locator("#elapsed-timer")
        self.time_start = page.locator("#time-start")
        self.time_end = page.locator("#time-end")

        # Red multi-selects
        self.sources_select = page.locator("#sources")
        self.targets_select = page.locator("#targets")
        self.tools_select = page.locator("#tools")

        # Red evidence
        self.red_files_input = page.locator("#redfiles")
        self.red_evidence_list = page.locator("#evidence-red")

        # Blue team fields
        self.blue_notes_textarea = page.locator("#bluenotes")
        self.blue_files_input = page.locator("#bluefiles")
        self.blue_evidence_list = page.locator("#evidence-blue")

        # Blue radio buttons
        self.prevented_yes = page.locator("#prevented-yes")
        self.prevented_partial = page.locator("#prevented-partial")
        self.prevented_no = page.locator("#prevented-no")
        self.prevented_na = page.locator("#prevented-na")

        self.priority_prevent = page.locator("#priority-prevent")
        self.priority_detect = page.locator("#priority-detect")
        self.priority_na = page.locator("#priority-na")

        self.alert_yes = page.locator("#alert-yes")
        self.alert_no = page.locator("#alert-no")

        self.log_yes = page.locator("#log-yes")
        self.log_no = page.locator("#log-no")

        # Blue conditional fields
        self.prevented_rating = page.locator("#preventedrating")
        self.prevention_source = page.locator("#preventionsource")
        self.same_source_checkbox = page.locator("#samesource")
        self.priority_urgency = page.locator("#priorityurgency")
        self.alert_severity = page.locator("#alertseverity")
        self.detection_rating = page.locator("#detectionrating")
        self.detection_source = page.locator("#detectionsource")

        # Blue conditional containers
        self.prevented_rating_container = page.locator("#preventedrating-container")
        self.prevention_source_container = page.locator("#preventionsource-container")
        self.urgency_container = page.locator("#urgency-container")
        self.alert_container = page.locator("#alert-container")
        self.logged_container = page.locator("#logged-container")
        self.detection_container = page.locator("#detection-container")
        self.detection_source_container = page.locator("#detectionsource-container")

        # Blue multi-selects
        self.controls_select = page.locator("#controls")
        self.tags_select = page.locator("#tags")
        self.datasources_select = page.locator("#datasources")
        self.rules_select = page.locator("#rules")

        # Multi-manage modals
        self.source_modal = page.locator("#multiSourceModal")
        self.target_modal = page.locator("#multiTargetModal")
        self.tool_modal = page.locator("#multiToolModal")
        self.control_modal = page.locator("#multiControlModal")
        self.tag_modal = page.locator("#multiTagModal")
        self.datasource_modal = page.locator("#multiDatasourceModal")
        self.rule_modal = page.locator("#multiRuleModal")
        self.detection_source_modal = page.locator("#multiDetectionsourceModal")
        self.prevention_source_modal = page.locator("#multiPreventionsourceModal")

        # TTP Info modal
        self.ttp_info_modal = page.locator("#ttpInfoModal")
        self.info_button = page.locator('[data-bs-target="#ttpInfoModal"]')

    def navigate(self, testcase_id: str):
        self.page.goto(f"/testcase/{testcase_id}")
        self.page.wait_for_selector("#ttpform")

    def save(self):
        with self.page.expect_response(lambda r: "/testcase/" in r.url and r.status == 200):
            self.save_button.click()
        self.page.wait_for_timeout(500)

    def go_back(self):
        self.back_button.click()

    def set_name(self, name: str):
        self.name_input.fill(name)

    def set_objective(self, text: str):
        self.objective_textarea.fill(text)

    def set_actions(self, text: str):
        self.actions_textarea.fill(text)

    def set_red_notes(self, text: str):
        self.red_notes_textarea.fill(text)

    def set_blue_notes(self, text: str):
        self.blue_notes_textarea.fill(text)

    def click_run_button(self):
        self.run_button.click()

    def upload_red_evidence(self, file_path: str):
        self.red_files_input.set_input_files(file_path)

    def upload_blue_evidence(self, file_path: str):
        self.blue_files_input.set_input_files(file_path)

    def delete_evidence(self, colour: str, filename: str):
        evidence_list = self.red_evidence_list if colour == "red" else self.blue_evidence_list
        delete_btn = evidence_list.locator(f'.evidence-delete:has-text("{filename}"), .evidence-delete').first
        with self.page.expect_response(lambda r: "evidence" in r.url):
            delete_btn.click()

    def download_evidence(self, filename: str):
        link = self.page.locator(f'a[href*="evidence/{filename}"]').first
        with self.page.expect_download() as download_info:
            link.click()
        return download_info.value

    def set_prevented(self, value: str):
        radio_map = {
            "Yes": self.prevented_yes,
            "Partial": self.prevented_partial,
            "No": self.prevented_no,
            "N/A": self.prevented_na,
        }
        radio_map[value].check(force=True)

    def set_priority(self, value: str):
        radio_map = {
            "Prevent": self.priority_prevent,
            "Detect": self.priority_detect,
            "N/A": self.priority_na,
        }
        radio_map[value].check(force=True)

    def set_alerted(self, value: str):
        radio_map = {"Yes": self.alert_yes, "No": self.alert_no}
        radio_map[value].check(force=True)

    def set_logged(self, value: str):
        radio_map = {"Yes": self.log_yes, "No": self.log_no}
        radio_map[value].check(force=True)

    def open_manage_modal(self, label_id: str):
        self.page.locator(f"#{label_id}").click()

    def add_multi_item(self, table_id: str, name: str, description: str = ""):
        new_btn = self.page.locator(f"#{table_id.replace('Table', '')}NewButton").first
        new_btn.click()
        last_row = self.page.locator(f"#{table_id} tbody tr").last
        last_row.locator('input[name="name"]').fill(name)
        if description:
            desc_input = last_row.locator('input[name="description"]')
            if desc_input.is_visible():
                desc_input.fill(description)

    def save_multi_modal(self):
        btn = self.page.locator(".multiButton:visible").first
        with self.page.expect_response(lambda r: "/multi/" in r.url):
            btn.click()
        self.page.wait_for_timeout(500)

    def open_ttp_info(self):
        self.info_button.click()
        expect(self.ttp_info_modal).to_be_visible()

    def expect_toast_visible(self):
        expect(self.toast).to_be_visible()

    def expect_state(self, state: str):
        expect(self.state_input).to_have_value(state)

    def expect_prevented_fields_visible(self):
        expect(self.prevented_rating_container).to_be_visible()

    def expect_prevented_fields_hidden(self):
        expect(self.prevented_rating_container).to_be_hidden()

    def expect_alert_container_visible(self):
        expect(self.alert_container).to_be_visible()

    def expect_alert_container_hidden(self):
        expect(self.alert_container).to_be_hidden()

    def expect_logged_container_visible(self):
        expect(self.logged_container).to_be_visible()

    def expect_logged_container_hidden(self):
        expect(self.logged_container).to_be_hidden()

    def expect_detection_container_visible(self):
        expect(self.detection_container).to_be_visible()

    def expect_detection_container_hidden(self):
        expect(self.detection_container).to_be_hidden()

    def expect_urgency_container_visible(self):
        expect(self.urgency_container).to_be_visible()

    def expect_urgency_container_hidden(self):
        expect(self.urgency_container).to_be_hidden()
