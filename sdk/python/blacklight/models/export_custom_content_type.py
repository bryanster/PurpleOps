from enum import Enum


class ExportCustomContentType(str, Enum):
    DETECTION_RULES = "detection_rules"
    NOTES = "notes"
    PROCEDURE_TEMPLATES = "procedure_templates"

    def __str__(self) -> str:
        return str(self.value)
