from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="PatchStep")


@_attrs_define
class PatchStep:
    """Every field is optional; only the ones present are changed. Soft freeze prevents changes to identity fields after
    the step's execution leaves pending.

    """

    name: str | Unset = UNSET
    objective: str | Unset = UNSET
    target_asset: str | Unset = UNSET
    tools: list[str] | Unset = UNSET
    controls_in_scope: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        objective = self.objective

        target_asset = self.target_asset

        tools: list[str] | Unset = UNSET
        if not isinstance(self.tools, Unset):
            tools = self.tools

        controls_in_scope: list[str] | Unset = UNSET
        if not isinstance(self.controls_in_scope, Unset):
            controls_in_scope = self.controls_in_scope

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if objective is not UNSET:
            field_dict["objective"] = objective
        if target_asset is not UNSET:
            field_dict["targetAsset"] = target_asset
        if tools is not UNSET:
            field_dict["tools"] = tools
        if controls_in_scope is not UNSET:
            field_dict["controlsInScope"] = controls_in_scope

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        objective = d.pop("objective", UNSET)

        target_asset = d.pop("targetAsset", UNSET)

        tools = cast(list[str], d.pop("tools", UNSET))

        controls_in_scope = cast(list[str], d.pop("controlsInScope", UNSET))

        patch_step = cls(
            name=name,
            objective=objective,
            target_asset=target_asset,
            tools=tools,
            controls_in_scope=controls_in_scope,
        )

        patch_step.additional_properties = d
        return patch_step

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
