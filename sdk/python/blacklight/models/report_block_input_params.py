from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ReportBlockInputParams")


@_attrs_define
class ReportBlockInputParams:
    """Block parameters, validated against the registry's ParamSchema.
    Defaults are used from the registry when absent.
    HTML content within params (e.g. rich_text `html`) is limited to
    100 KiB raw and sanitized server-side on write (M6-005).

    Parameter names come from the block's own ParamSchema and are not
    interchangeable: a key the schema does not declare is rejected, so
    a client that misspells one fails the whole save.

    """

    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        report_block_input_params = cls()

        report_block_input_params.additional_properties = d
        return report_block_input_params

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
