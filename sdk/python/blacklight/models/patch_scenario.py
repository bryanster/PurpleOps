from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="PatchScenario")


@_attrs_define
class PatchScenario:
    """Every field is optional; only the ones present are changed."""

    name: str | Unset = UNSET
    narrative: str | Unset = UNSET
    threat_actor: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        narrative = self.narrative

        threat_actor = self.threat_actor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if narrative is not UNSET:
            field_dict["narrative"] = narrative
        if threat_actor is not UNSET:
            field_dict["threatActor"] = threat_actor

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        narrative = d.pop("narrative", UNSET)

        threat_actor = d.pop("threatActor", UNSET)

        patch_scenario = cls(
            name=name,
            narrative=narrative,
            threat_actor=threat_actor,
        )

        patch_scenario.additional_properties = d
        return patch_scenario

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
