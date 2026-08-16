from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="NavigatorLayerVersions")


@_attrs_define
class NavigatorLayerVersions:
    attack: str
    navigator: str
    layer: str

    def to_dict(self) -> dict[str, Any]:
        attack = self.attack

        navigator = self.navigator

        layer = self.layer

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "attack": attack,
                "navigator": navigator,
                "layer": layer,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        attack = d.pop("attack")

        navigator = d.pop("navigator")

        layer = d.pop("layer")

        navigator_layer_versions = cls(
            attack=attack,
            navigator=navigator,
            layer=layer,
        )

        return navigator_layer_versions
