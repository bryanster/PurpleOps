from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from uuid import UUID


T = TypeVar("T", bound="ClaimReportShareResult")


@_attrs_define
class ClaimReportShareResult:
    version_id: UUID
    report_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        version_id = str(self.version_id)

        report_id: str | Unset = UNSET
        if not isinstance(self.report_id, Unset):
            report_id = str(self.report_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "versionId": version_id,
            }
        )
        if report_id is not UNSET:
            field_dict["reportId"] = report_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version_id = UUID(d.pop("versionId"))

        _report_id = d.pop("reportId", UNSET)
        report_id: UUID | Unset
        if isinstance(_report_id, Unset):
            report_id = UNSET
        else:
            report_id = UUID(_report_id)

        claim_report_share_result = cls(
            version_id=version_id,
            report_id=report_id,
        )

        return claim_report_share_result
