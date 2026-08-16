from enum import Enum


class EngagementStatus(str, Enum):
    ACTIVE = "active"
    ARCHIVED = "archived"
    CLOSED = "closed"
    DRAFT = "draft"

    def __str__(self) -> str:
        return str(self.value)
