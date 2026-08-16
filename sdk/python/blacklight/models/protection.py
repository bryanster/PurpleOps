from enum import Enum


class Protection(str, Enum):
    BLOCKED = "blocked"
    NA = "n/a"
    NOT_BLOCKED = "not_blocked"
    PARTIAL = "partial"

    def __str__(self) -> str:
        return str(self.value)
