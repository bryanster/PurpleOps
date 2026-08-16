from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.scenario_source import ScenarioSource
from ..types import UNSET, Unset


T = TypeVar("T", bound="CreateScenario")


@_attrs_define
class CreateScenario:
    name: str
    narrative: str | Unset = ""
    threat_actor: str | Unset = ""
    source: ScenarioSource | Unset = UNSET
    """ Provenance of a scenario. `manual` is authored here; `ctid` and `imported` come from content. """
    source_ref: str | Unset = ""
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        narrative = self.narrative

        threat_actor = self.threat_actor

        source: str | Unset = UNSET
        if not isinstance(self.source, Unset):
            source = self.source.value

        source_ref = self.source_ref

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
            }
        )
        if narrative is not UNSET:
            field_dict["narrative"] = narrative
        if threat_actor is not UNSET:
            field_dict["threatActor"] = threat_actor
        if source is not UNSET:
            field_dict["source"] = source
        if source_ref is not UNSET:
            field_dict["sourceRef"] = source_ref

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name")

        narrative = d.pop("narrative", UNSET)

        threat_actor = d.pop("threatActor", UNSET)

        _source = d.pop("source", UNSET)
        source: ScenarioSource | Unset
        if isinstance(_source, Unset):
            source = UNSET
        else:
            source = ScenarioSource(_source)

        source_ref = d.pop("sourceRef", UNSET)

        create_scenario = cls(
            name=name,
            narrative=narrative,
            threat_actor=threat_actor,
            source=source,
            source_ref=source_ref,
        )

        create_scenario.additional_properties = d
        return create_scenario

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
