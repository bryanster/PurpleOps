from enum import Enum


class UserStatus(str, Enum):
    ACTIVE = "active"
    DISABLED = "disabled"
    INVITED = "invited"

    def __str__(self) -> str:
        return str(self.value)
