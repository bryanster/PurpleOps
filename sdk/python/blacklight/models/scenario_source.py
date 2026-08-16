from enum import Enum


class ScenarioSource(str, Enum):
    CTID = "ctid"
    IMPORTED = "imported"
    MANUAL = "manual"

    def __str__(self) -> str:
        return str(self.value)
