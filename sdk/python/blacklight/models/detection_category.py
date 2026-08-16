from enum import Enum


class DetectionCategory(str, Enum):
    GENERAL = "general"
    NONE = "none"
    TACTIC = "tactic"
    TECHNIQUE = "technique"
    TELEMETRY = "telemetry"

    def __str__(self) -> str:
        return str(self.value)
