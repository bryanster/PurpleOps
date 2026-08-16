from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="NavigatorLegendItem")


@_attrs_define
class NavigatorLegendItem:
    label: str
    color: str

    def to_dict(self) -> dict[str, Any]:
        label = self.label

        color = self.color

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "label": label,
                "color": color,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        label = d.pop("label")

        color = d.pop("color")

        navigator_legend_item = cls(
            label=label,
            color=color,
        )

        return navigator_legend_item
