from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast


T = TypeVar("T", bound="NavigatorLayerFilters")


@_attrs_define
class NavigatorLayerFilters:
    platforms: list[str]

    def to_dict(self) -> dict[str, Any]:
        platforms = self.platforms

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "platforms": platforms,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        platforms = cast(list[str], d.pop("platforms"))

        navigator_layer_filters = cls(
            platforms=platforms,
        )

        return navigator_layer_filters
