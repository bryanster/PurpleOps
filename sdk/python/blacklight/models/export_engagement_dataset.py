from enum import Enum


class ExportEngagementDataset(str, Enum):
    COVERAGE = "coverage"
    EXECUTIONS = "executions"
    FINDINGS = "findings"

    def __str__(self) -> str:
        return str(self.value)
