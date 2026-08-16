from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ContentTechniqueDetail")


@_attrs_define
class ContentTechniqueDetail:
    id: UUID
    source_id: UUID
    version: str
    """ ATT&CK release label (for example `15.1`). """
    external_id: str
    """ MITRE id (`T1059`, `T1059.001`). """
    name: str
    description: str
    is_subtechnique: bool
    parent_external_id: str
    """ Parent technique MITRE id when `isSubtechnique` is true; empty otherwise. """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    tactics: list[str]
    """ Tactic external ids this technique maps to. """
    mitigations: list[str]
    """ Mitigation external ids that mitigate this technique. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        source_id = str(self.source_id)

        version = self.version

        external_id = self.external_id

        name = self.name

        description = self.description

        is_subtechnique = self.is_subtechnique

        parent_external_id = self.parent_external_id

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        tactics = self.tactics

        mitigations = self.mitigations

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "sourceId": source_id,
                "version": version,
                "externalId": external_id,
                "name": name,
                "description": description,
                "isSubtechnique": is_subtechnique,
                "parentExternalId": parent_external_id,
                "createdAt": created_at,
                "updatedAt": updated_at,
                "tactics": tactics,
                "mitigations": mitigations,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        source_id = UUID(d.pop("sourceId"))

        version = d.pop("version")

        external_id = d.pop("externalId")

        name = d.pop("name")

        description = d.pop("description")

        is_subtechnique = d.pop("isSubtechnique")

        parent_external_id = d.pop("parentExternalId")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        tactics = cast(list[str], d.pop("tactics"))

        mitigations = cast(list[str], d.pop("mitigations"))

        content_technique_detail = cls(
            id=id,
            source_id=source_id,
            version=version,
            external_id=external_id,
            name=name,
            description=description,
            is_subtechnique=is_subtechnique,
            parent_external_id=parent_external_id,
            created_at=created_at,
            updated_at=updated_at,
            tactics=tactics,
            mitigations=mitigations,
        )

        content_technique_detail.additional_properties = d
        return content_technique_detail

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
