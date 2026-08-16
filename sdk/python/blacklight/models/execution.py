from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.execution_detection_category import ExecutionDetectionCategory
from ..models.execution_outcome import ExecutionOutcome
from ..models.execution_protection import ExecutionProtection
from ..models.execution_status import ExecutionStatus
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Execution")


@_attrs_define
class Execution:
    id: UUID
    """ UUIDv7. """
    step_id: UUID
    version: int
    """ Optimistic-lock version. Incremented on every red or blue PATCH. """
    status: ExecutionStatus
    """ The red-side state of one execution. """
    executed_by: str
    """ User id of the first red operator who moved this out of pending. """
    command_run: str
    """ The command or payload that was executed. """
    source_host: str
    """ The host that ran the attack. """
    target_host: str
    """ The host that received the attack. """
    red_notes: str
    """ Free-form notes from the red operator. """
    detection_modifiers: list[str]
    """ Qualifiers on the detection category. """
    detecting_source: str
    """ Source of detection (e.g. "Splunk", "Sentinel"). """
    detecting_rule_ref: str
    """ Reference to the detection rule that fired. """
    alert_severity: str
    """ Alert severity (e.g. "high", "medium", "low"). """
    blue_notes: str
    """ Free-form notes from the blue operator. """
    scored_by: str
    """ User id of the scorer. """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    started_at: datetime.datetime | Unset = UNSET
    ended_at: datetime.datetime | Unset = UNSET
    detection_category: ExecutionDetectionCategory | Unset = UNSET
    protection: ExecutionProtection | Unset = UNSET
    detected_at: datetime.datetime | Unset = UNSET
    scored_at: datetime.datetime | Unset = UNSET
    outcome: ExecutionOutcome | Unset = UNSET
    """ Derived detection outcome from category × protection. Never stored; computed on read. """
    mttd_seconds: int | None | Unset = UNSET
    """ Mean time to detect in seconds (detected_at − started_at). Computed on read; null when either timestamp is
    missing. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        step_id = str(self.step_id)

        version = self.version

        status = self.status.value

        executed_by = self.executed_by

        command_run = self.command_run

        source_host = self.source_host

        target_host = self.target_host

        red_notes = self.red_notes

        detection_modifiers = self.detection_modifiers

        detecting_source = self.detecting_source

        detecting_rule_ref = self.detecting_rule_ref

        alert_severity = self.alert_severity

        blue_notes = self.blue_notes

        scored_by = self.scored_by

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        ended_at: str | Unset = UNSET
        if not isinstance(self.ended_at, Unset):
            ended_at = self.ended_at.isoformat()

        detection_category: str | Unset = UNSET
        if not isinstance(self.detection_category, Unset):
            detection_category = self.detection_category.value

        protection: str | Unset = UNSET
        if not isinstance(self.protection, Unset):
            protection = self.protection.value

        detected_at: str | Unset = UNSET
        if not isinstance(self.detected_at, Unset):
            detected_at = self.detected_at.isoformat()

        scored_at: str | Unset = UNSET
        if not isinstance(self.scored_at, Unset):
            scored_at = self.scored_at.isoformat()

        outcome: str | Unset = UNSET
        if not isinstance(self.outcome, Unset):
            outcome = self.outcome.value

        mttd_seconds: int | None | Unset
        if isinstance(self.mttd_seconds, Unset):
            mttd_seconds = UNSET
        else:
            mttd_seconds = self.mttd_seconds

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "stepId": step_id,
                "version": version,
                "status": status,
                "executedBy": executed_by,
                "commandRun": command_run,
                "sourceHost": source_host,
                "targetHost": target_host,
                "redNotes": red_notes,
                "detectionModifiers": detection_modifiers,
                "detectingSource": detecting_source,
                "detectingRuleRef": detecting_rule_ref,
                "alertSeverity": alert_severity,
                "blueNotes": blue_notes,
                "scoredBy": scored_by,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )
        if started_at is not UNSET:
            field_dict["startedAt"] = started_at
        if ended_at is not UNSET:
            field_dict["endedAt"] = ended_at
        if detection_category is not UNSET:
            field_dict["detectionCategory"] = detection_category
        if protection is not UNSET:
            field_dict["protection"] = protection
        if detected_at is not UNSET:
            field_dict["detectedAt"] = detected_at
        if scored_at is not UNSET:
            field_dict["scoredAt"] = scored_at
        if outcome is not UNSET:
            field_dict["outcome"] = outcome
        if mttd_seconds is not UNSET:
            field_dict["mttdSeconds"] = mttd_seconds

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        step_id = UUID(d.pop("stepId"))

        version = d.pop("version")

        status = ExecutionStatus(d.pop("status"))

        executed_by = d.pop("executedBy")

        command_run = d.pop("commandRun")

        source_host = d.pop("sourceHost")

        target_host = d.pop("targetHost")

        red_notes = d.pop("redNotes")

        detection_modifiers = cast(list[str], d.pop("detectionModifiers"))

        detecting_source = d.pop("detectingSource")

        detecting_rule_ref = d.pop("detectingRuleRef")

        alert_severity = d.pop("alertSeverity")

        blue_notes = d.pop("blueNotes")

        scored_by = d.pop("scoredBy")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        _started_at = d.pop("startedAt", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _ended_at = d.pop("endedAt", UNSET)
        ended_at: datetime.datetime | Unset
        if isinstance(_ended_at, Unset):
            ended_at = UNSET
        else:
            ended_at = datetime.datetime.fromisoformat(_ended_at)

        _detection_category = d.pop("detectionCategory", UNSET)
        detection_category: ExecutionDetectionCategory | Unset
        if isinstance(_detection_category, Unset):
            detection_category = UNSET
        else:
            detection_category = ExecutionDetectionCategory(_detection_category)

        _protection = d.pop("protection", UNSET)
        protection: ExecutionProtection | Unset
        if isinstance(_protection, Unset):
            protection = UNSET
        else:
            protection = ExecutionProtection(_protection)

        _detected_at = d.pop("detectedAt", UNSET)
        detected_at: datetime.datetime | Unset
        if isinstance(_detected_at, Unset):
            detected_at = UNSET
        else:
            detected_at = datetime.datetime.fromisoformat(_detected_at)

        _scored_at = d.pop("scoredAt", UNSET)
        scored_at: datetime.datetime | Unset
        if isinstance(_scored_at, Unset):
            scored_at = UNSET
        else:
            scored_at = datetime.datetime.fromisoformat(_scored_at)

        _outcome = d.pop("outcome", UNSET)
        outcome: ExecutionOutcome | Unset
        if isinstance(_outcome, Unset):
            outcome = UNSET
        else:
            outcome = ExecutionOutcome(_outcome)

        def _parse_mttd_seconds(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        mttd_seconds = _parse_mttd_seconds(d.pop("mttdSeconds", UNSET))

        execution = cls(
            id=id,
            step_id=step_id,
            version=version,
            status=status,
            executed_by=executed_by,
            command_run=command_run,
            source_host=source_host,
            target_host=target_host,
            red_notes=red_notes,
            detection_modifiers=detection_modifiers,
            detecting_source=detecting_source,
            detecting_rule_ref=detecting_rule_ref,
            alert_severity=alert_severity,
            blue_notes=blue_notes,
            scored_by=scored_by,
            created_at=created_at,
            updated_at=updated_at,
            started_at=started_at,
            ended_at=ended_at,
            detection_category=detection_category,
            protection=protection,
            detected_at=detected_at,
            scored_at=scored_at,
            outcome=outcome,
            mttd_seconds=mttd_seconds,
        )

        execution.additional_properties = d
        return execution

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
