from enum import Enum


class EngagementMode(str, Enum):
    BLIND = "blind"
    STANDARD = "standard"

    def __str__(self) -> str:
        return str(self.value)
