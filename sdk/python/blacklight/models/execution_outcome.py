from enum import Enum


class ExecutionOutcome(str, Enum):
    DETECTED = "detected"
    NOT_APPLICABLE = "not_applicable"
    NOT_DETECTED = "not_detected"
    PREVENTED = "prevented"

    def __str__(self) -> str:
        return str(self.value)
