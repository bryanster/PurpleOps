from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="SeverityBucket")


@_attrs_define
class SeverityBucket:
    severity: str
    open_: int
    in_progress: int
    resolved: int
    accepted_risk: int
    total_open: int

    def to_dict(self) -> dict[str, Any]:
        severity = self.severity

        open_ = self.open_

        in_progress = self.in_progress

        resolved = self.resolved

        accepted_risk = self.accepted_risk

        total_open = self.total_open

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "severity": severity,
                "open": open_,
                "inProgress": in_progress,
                "resolved": resolved,
                "acceptedRisk": accepted_risk,
                "totalOpen": total_open,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        severity = d.pop("severity")

        open_ = d.pop("open")

        in_progress = d.pop("inProgress")

        resolved = d.pop("resolved")

        accepted_risk = d.pop("acceptedRisk")

        total_open = d.pop("totalOpen")

        severity_bucket = cls(
            severity=severity,
            open_=open_,
            in_progress=in_progress,
            resolved=resolved,
            accepted_risk=accepted_risk,
            total_open=total_open,
        )

        return severity_bucket
