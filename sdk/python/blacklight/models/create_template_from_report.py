from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from uuid import UUID


T = TypeVar("T", bound="CreateTemplateFromReport")


@_attrs_define
class CreateTemplateFromReport:
    report_id: UUID
    name: str

    def to_dict(self) -> dict[str, Any]:
        report_id = str(self.report_id)

        name = self.name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "reportId": report_id,
                "name": name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        report_id = UUID(d.pop("reportId"))

        name = d.pop("name")

        create_template_from_report = cls(
            report_id=report_id,
            name=name,
        )

        return create_template_from_report
