"""Page Object Model for the Assessment Detail page."""

from playwright.sync_api import Page, expect


class AssessmentDetailPage:

    def __init__(self, page: Page):
        self.page = page
        self.table = page.locator("#assessmentTable")
        self.toolbar = page.locator("#toolbar")
        self.new_testcase_btn = page.locator("#newTestcase")
        self.selected_count = page.locator("#selected-count")

        # New Testcase modal
        self.new_tc_modal = page.locator("#newTestcaseModal")
        self.tc_name_input = page.locator("#newTestcaseModal #name")
        self.tc_mitreid_select = page.locator("#newTestcaseModal #mitreid")
        self.tc_tactic_select = page.locator("#newTestcaseModal #tactic")
        self.tc_submit_btn = page.locator("#newTestcaseButton")

        # Templates modal
        self.templates_modal = page.locator("#testcaseTemplatesModal")
        self.templates_table = page.locator("#testcaseTemplateTable")
        self.templates_submit_btn = page.locator("#testcaseTemplatesButton")

        # Navigator import modal
        self.navigator_modal = page.locator("#testcaseNavigatorModal")
        self.navigator_file_input = page.locator("#testcaseNavigatorModal #formFile")

        # Campaign import modal
        self.campaign_modal = page.locator("#testcaseCampaignModal")
        self.campaign_file_input = page.locator("#testcaseCampaignModal #campaignFile")

        # Manage modals
        self.manage_datasources_modal = page.locator("#manageDatasourcesModal")
        self.manage_rules_modal = page.locator("#manageRulesModal")
        self.manage_detection_modal = page.locator("#manageDetectionsourcesModal")
        self.manage_prevention_modal = page.locator("#managePreventionsourcesModal")

        # Export report modal
        self.export_report_modal = page.locator("#exportReportModal")

    def navigate(self, assessment_id: str):
        self.page.goto(f"/assessment/{assessment_id}")
        self.page.wait_for_selector("#assessmentTable")

    def create_testcase(self, name: str, mitre_id: str = None, tactic: str = None):
        self.new_testcase_btn.click()
        expect(self.new_tc_modal).to_be_visible()
        self.tc_name_input.fill(name)
        if mitre_id:
            self._select_bootstrap_select("#newTestcaseModal #mitreid", mitre_id)
        if tactic:
            self.tc_tactic_select.select_option(label=tactic)
        with self.page.expect_response(lambda r: "/single" in r.url and r.status == 200):
            self.tc_submit_btn.click()
        self.page.wait_for_timeout(500)

    def _select_bootstrap_select(self, selector: str, value: str):
        container = self.page.locator(selector).locator("..").locator("..")
        container.locator(".dropdown-toggle").click()
        search = container.locator('.bs-searchbox input[type="search"]')
        if search.is_visible():
            search.fill(value)
        container.locator(f'.dropdown-item:has-text("{value}")').first.click()

    def get_testcase_count(self) -> int:
        return self.table.locator("tbody tr").count()

    def click_testcase_link(self, name: str):
        self.table.locator(f'a:has-text("{name}")').first.click()

    def toggle_visibility(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        btn = row.locator('[onclick*="visibleTest"]')
        with self.page.expect_response(lambda r: "toggle-visibility" in r.url):
            btn.click()

    def clone_testcase(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        btn = row.locator('[onclick*="cloneTest"]')
        with self.page.expect_response(lambda r: "clone" in r.url):
            btn.click()
        self.page.wait_for_timeout(500)

    def delete_testcase(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        btn = row.locator('[onclick*="deleteTest"]')
        with self.page.expect_response(lambda r: "delete" in r.url):
            btn.click()
        self.page.wait_for_timeout(500)

    def toggle_timer(self, row_index: int):
        row = self.table.locator("tbody tr").nth(row_index)
        btn = row.locator('[onclick*="toggleTimer"]')
        with self.page.expect_response(lambda r: "toggle-timer" in r.url):
            btn.click()

    def search_table(self, query: str):
        search = self.page.locator('.search-input input[type="search"]')
        search.fill(query)
        self.page.wait_for_timeout(500)

    def select_row_checkbox(self, row_index: int):
        self.table.locator("tbody tr").nth(row_index).locator('input[type="checkbox"]').check()

    def open_manage_modal(self, modal_type: str):
        """Open a manage modal: datasources, rules, detectionsources, preventionsources."""
        manage_dropdown = self.toolbar.locator('button:has-text("Manage")')
        manage_dropdown.click()
        self.page.locator(f'[data-bs-target="#{modal_type}Modal"], a:has-text("{modal_type}")').first.click()

    def add_manage_item(self, table_id: str, name: str, description: str = ""):
        new_btn = self.page.locator(f"#{table_id}Toolbar .multiNew, #{table_id.replace('Table', '')}NewButton").first
        new_btn.click()
        rows = self.page.locator(f"#{table_id} tbody tr")
        last_row = rows.last
        last_row.locator('input[name="name"]').fill(name)
        if description:
            desc_input = last_row.locator('input[name="description"]')
            if desc_input.is_visible():
                desc_input.fill(description)

    def save_manage_modal(self):
        btn = self.page.locator(".assessmentMultiButton:visible").first
        with self.page.expect_response(lambda r: "/multi/" in r.url):
            btn.click()
        self.page.wait_for_timeout(500)

    def import_from_templates(self, template_indices: list[int] = None):
        """Open templates modal and select templates by row index."""
        import_dropdown = self.toolbar.locator('button:has-text("Import")')
        import_dropdown.click()
        self.page.locator('a:has-text("Testcase(s) From Template")').click()
        expect(self.templates_modal).to_be_visible()
        if template_indices:
            for idx in template_indices:
                self.templates_table.locator("tbody tr").nth(idx).locator('input[type="checkbox"]').check()
        with self.page.expect_response(lambda r: "import/template" in r.url):
            self.templates_submit_btn.click()
        self.page.wait_for_timeout(500)

    def import_navigator_layer(self, file_path: str):
        import_dropdown = self.toolbar.locator('button:has-text("Import")')
        import_dropdown.click()
        self.page.locator('a:has-text("MITRE ATT&CK Navigator Layer")').click()
        expect(self.navigator_modal).to_be_visible()
        self.navigator_file_input.set_input_files(file_path)
        submit = self.navigator_modal.locator('button[type="submit"]')
        with self.page.expect_response(lambda r: "import/navigator" in r.url):
            submit.click()
        self.page.wait_for_timeout(500)

    def import_campaign(self, file_path: str):
        import_dropdown = self.toolbar.locator('button:has-text("Import")')
        import_dropdown.click()
        self.page.locator('a:has-text("Campaign Template")').click()
        expect(self.campaign_modal).to_be_visible()
        self.campaign_file_input.set_input_files(file_path)
        submit = self.campaign_modal.locator('button[type="submit"]')
        with self.page.expect_response(lambda r: "import/campaign" in r.url):
            submit.click()
        self.page.wait_for_timeout(500)

    def click_export(self, export_type: str):
        """Click an export link. Returns download for file exports."""
        export_dropdown = self.toolbar.locator('button:has-text("Export")')
        export_dropdown.click()
        link = self.page.locator(f'a:has-text("{export_type}")').first
        with self.page.expect_download() as download_info:
            link.click()
        return download_info.value

    def click_statistics(self):
        self.page.locator('a:has-text("Statistics")').click()

    def click_navigator(self):
        self.page.locator('a:has-text("ATT&CK Navigator")').click()

    def expect_testcase_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_be_visible()

    def expect_testcase_not_in_table(self, name: str):
        expect(self.table.locator(f'td:has-text("{name}")')).to_have_count(0)

    def expect_hexagon_chart(self):
        expect(self.page.locator("svg")).to_be_visible()
