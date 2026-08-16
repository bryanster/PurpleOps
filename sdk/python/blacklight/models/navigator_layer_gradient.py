from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast


T = TypeVar("T", bound="NavigatorLayerGradient")


@_attrs_define
class NavigatorLayerGradient:
    colors: list[str]
    min_value: int
    max_value: int

    def to_dict(self) -> dict[str, Any]:
        colors = self.colors

        min_value = self.min_value

        max_value = self.max_value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "colors": colors,
                "minValue": min_value,
                "maxValue": max_value,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        colors = cast(list[str], d.pop("colors"))

        min_value = d.pop("minValue")

        max_value = d.pop("maxValue")

        navigator_layer_gradient = cls(
            colors=colors,
            min_value=min_value,
            max_value=max_value,
        )

        return navigator_layer_gradient
