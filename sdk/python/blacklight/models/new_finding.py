from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.finding_severity import FindingSeverity
from ..models.finding_status import FindingStatus
from ..types import UNSET, Unset
from uuid import UUID


T = TypeVar("T", bound="NewFinding")


@_attrs_define
class NewFinding:
    title: str
    description: str
    severity: FindingSeverity
    """ Severity of a finding. """
    recommendation: str | Unset = UNSET
    owner: str | Unset = UNSET
    """ User id of the owner; defaults to caller when empty. """
    status: FindingStatus | Unset = UNSET
    """ Lifecycle of a remediation finding. """
    created_from_execution: UUID | Unset = UNSET
    """ Optional execution id this finding originates from. """

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        description = self.description

        severity = self.severity.value

        recommendation = self.recommendation

        owner = self.owner

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        created_from_execution: str | Unset = UNSET
        if not isinstance(self.created_from_execution, Unset):
            created_from_execution = str(self.created_from_execution)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "title": title,
                "description": description,
                "severity": severity,
            }
        )
        if recommendation is not UNSET:
            field_dict["recommendation"] = recommendation
        if owner is not UNSET:
            field_dict["owner"] = owner
        if status is not UNSET:
            field_dict["status"] = status
        if created_from_execution is not UNSET:
            field_dict["createdFromExecution"] = created_from_execution

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        title = d.pop("title")

        description = d.pop("description")

        severity = FindingSeverity(d.pop("severity"))

        recommendation = d.pop("recommendation", UNSET)

        owner = d.pop("owner", UNSET)

        _status = d.pop("status", UNSET)
        status: FindingStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = FindingStatus(_status)

        _created_from_execution = d.pop("createdFromExecution", UNSET)
        created_from_execution: UUID | Unset
        if isinstance(_created_from_execution, Unset):
            created_from_execution = UNSET
        else:
            created_from_execution = UUID(_created_from_execution)

        new_finding = cls(
            title=title,
            description=description,
            severity=severity,
            recommendation=recommendation,
            owner=owner,
            status=status,
            created_from_execution=created_from_execution,
        )

        return new_finding
