from enum import Enum


class EngagementRole(str, Enum):
    BLUE = "blue"
    LEAD = "lead"
    OBSERVER = "observer"
    RED = "red"

    def __str__(self) -> str:
        return str(self.value)
