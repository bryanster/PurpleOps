from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.finding_severity import FindingSeverity
from ..models.finding_status import FindingStatus
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Finding")


@_attrs_define
class Finding:
    id: UUID
    engagement_id: UUID
    title: str
    description: str
    severity: FindingSeverity
    """ Severity of a finding. """
    recommendation: str
    owner: str
    """ User id of the owner, or empty string. """
    status: FindingStatus
    """ Lifecycle of a remediation finding. """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    step_ids: list[UUID]
    """ Step ids linked to this finding. """
    created_from_execution: None | Unset | UUID = UNSET
    """ The execution this finding was created from, if any. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        engagement_id = str(self.engagement_id)

        title = self.title

        description = self.description

        severity = self.severity.value

        recommendation = self.recommendation

        owner = self.owner

        status = self.status.value

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        step_ids = []
        for step_ids_item_data in self.step_ids:
            step_ids_item = str(step_ids_item_data)
            step_ids.append(step_ids_item)

        created_from_execution: None | str | Unset
        if isinstance(self.created_from_execution, Unset):
            created_from_execution = UNSET
        elif isinstance(self.created_from_execution, UUID):
            created_from_execution = str(self.created_from_execution)
        else:
            created_from_execution = self.created_from_execution

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "engagementId": engagement_id,
                "title": title,
                "description": description,
                "severity": severity,
                "recommendation": recommendation,
                "owner": owner,
                "status": status,
                "createdAt": created_at,
                "updatedAt": updated_at,
                "stepIds": step_ids,
            }
        )
        if created_from_execution is not UNSET:
            field_dict["createdFromExecution"] = created_from_execution

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        engagement_id = UUID(d.pop("engagementId"))

        title = d.pop("title")

        description = d.pop("description")

        severity = FindingSeverity(d.pop("severity"))

        recommendation = d.pop("recommendation")

        owner = d.pop("owner")

        status = FindingStatus(d.pop("status"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        step_ids = []
        _step_ids = d.pop("stepIds")
        for step_ids_item_data in _step_ids:
            step_ids_item = UUID(step_ids_item_data)

            step_ids.append(step_ids_item)

        def _parse_created_from_execution(data: object) -> None | Unset | UUID:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                created_from_execution_type_0 = UUID(data)

                return created_from_execution_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | UUID, data)

        created_from_execution = _parse_created_from_execution(d.pop("createdFromExecution", UNSET))

        finding = cls(
            id=id,
            engagement_id=engagement_id,
            title=title,
            description=description,
            severity=severity,
            recommendation=recommendation,
            owner=owner,
            status=status,
            created_at=created_at,
            updated_at=updated_at,
            step_ids=step_ids,
            created_from_execution=created_from_execution,
        )

        finding.additional_properties = d
        return finding

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
