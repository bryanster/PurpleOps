from enum import Enum


class CreateUserRequestStatus(str, Enum):
    ACTIVE = "active"
    INVITED = "invited"

    def __str__(self) -> str:
        return str(self.value)
