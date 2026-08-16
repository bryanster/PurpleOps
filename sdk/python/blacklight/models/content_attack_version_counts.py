from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ContentAttackVersionCounts")


@_attrs_define
class ContentAttackVersionCounts:
    """Object counts by family for one ATT&CK version."""

    tactics: int
    techniques: int
    mitigations: int
    groups: int
    software: int
    data_sources: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        tactics = self.tactics

        techniques = self.techniques

        mitigations = self.mitigations

        groups = self.groups

        software = self.software

        data_sources = self.data_sources

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "tactics": tactics,
                "techniques": techniques,
                "mitigations": mitigations,
                "groups": groups,
                "software": software,
                "dataSources": data_sources,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        tactics = d.pop("tactics")

        techniques = d.pop("techniques")

        mitigations = d.pop("mitigations")

        groups = d.pop("groups")

        software = d.pop("software")

        data_sources = d.pop("dataSources")

        content_attack_version_counts = cls(
            tactics=tactics,
            techniques=techniques,
            mitigations=mitigations,
            groups=groups,
            software=software,
            data_sources=data_sources,
        )

        content_attack_version_counts.additional_properties = d
        return content_attack_version_counts

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
