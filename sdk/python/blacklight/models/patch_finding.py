from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.finding_severity import FindingSeverity
from ..models.finding_status import FindingStatus
from ..types import UNSET, Unset


T = TypeVar("T", bound="PatchFinding")


@_attrs_define
class PatchFinding:
    title: str | Unset = UNSET
    description: str | Unset = UNSET
    severity: FindingSeverity | Unset = UNSET
    """ Severity of a finding. """
    recommendation: str | Unset = UNSET
    owner: str | Unset = UNSET
    status: FindingStatus | Unset = UNSET
    """ Lifecycle of a remediation finding. """

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        description = self.description

        severity: str | Unset = UNSET
        if not isinstance(self.severity, Unset):
            severity = self.severity.value

        recommendation = self.recommendation

        owner = self.owner

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if title is not UNSET:
            field_dict["title"] = title
        if description is not UNSET:
            field_dict["description"] = description
        if severity is not UNSET:
            field_dict["severity"] = severity
        if recommendation is not UNSET:
            field_dict["recommendation"] = recommendation
        if owner is not UNSET:
            field_dict["owner"] = owner
        if status is not UNSET:
            field_dict["status"] = status

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        title = d.pop("title", UNSET)

        description = d.pop("description", UNSET)

        _severity = d.pop("severity", UNSET)
        severity: FindingSeverity | Unset
        if isinstance(_severity, Unset):
            severity = UNSET
        else:
            severity = FindingSeverity(_severity)

        recommendation = d.pop("recommendation", UNSET)

        owner = d.pop("owner", UNSET)

        _status = d.pop("status", UNSET)
        status: FindingStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = FindingStatus(_status)

        patch_finding = cls(
            title=title,
            description=description,
            severity=severity,
            recommendation=recommendation,
            owner=owner,
            status=status,
        )

        return patch_finding
