from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from uuid import UUID


T = TypeVar("T", bound="ApplyTemplate")


@_attrs_define
class ApplyTemplate:
    template_id: UUID

    def to_dict(self) -> dict[str, Any]:
        template_id = str(self.template_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "templateId": template_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        template_id = UUID(d.pop("templateId"))

        apply_template = cls(
            template_id=template_id,
        )

        return apply_template
