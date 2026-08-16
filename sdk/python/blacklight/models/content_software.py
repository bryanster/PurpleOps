from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.content_software_type import ContentSoftwareType
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ContentSoftware")


@_attrs_define
class ContentSoftware:
    id: UUID
    source_id: UUID
    version: str
    external_id: str
    name: str
    description: str
    software_type: ContentSoftwareType
    created_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        source_id = str(self.source_id)

        version = self.version

        external_id = self.external_id

        name = self.name

        description = self.description

        software_type = self.software_type.value

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

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
                "softwareType": software_type,
                "createdAt": created_at,
                "updatedAt": updated_at,
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

        software_type = ContentSoftwareType(d.pop("softwareType"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        content_software = cls(
            id=id,
            source_id=source_id,
            version=version,
            external_id=external_id,
            name=name,
            description=description,
            software_type=software_type,
            created_at=created_at,
            updated_at=updated_at,
        )

        content_software.additional_properties = d
        return content_software

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
