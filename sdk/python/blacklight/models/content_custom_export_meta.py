from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="ContentCustomExportMeta")


@_attrs_define
class ContentCustomExportMeta:
    """License/attribution header for a custom content export."""

    source_name: str
    attribution: str
    exported_at: datetime.datetime
    license_spdx: str | Unset = UNSET
    license_name: str | Unset = UNSET
    license_url: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        source_name = self.source_name

        attribution = self.attribution

        exported_at = self.exported_at.isoformat()

        license_spdx = self.license_spdx

        license_name = self.license_name

        license_url = self.license_url

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "sourceName": source_name,
                "attribution": attribution,
                "exportedAt": exported_at,
            }
        )
        if license_spdx is not UNSET:
            field_dict["licenseSpdx"] = license_spdx
        if license_name is not UNSET:
            field_dict["licenseName"] = license_name
        if license_url is not UNSET:
            field_dict["licenseUrl"] = license_url

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        source_name = d.pop("sourceName")

        attribution = d.pop("attribution")

        exported_at = datetime.datetime.fromisoformat(d.pop("exportedAt"))

        license_spdx = d.pop("licenseSpdx", UNSET)

        license_name = d.pop("licenseName", UNSET)

        license_url = d.pop("licenseUrl", UNSET)

        content_custom_export_meta = cls(
            source_name=source_name,
            attribution=attribution,
            exported_at=exported_at,
            license_spdx=license_spdx,
            license_name=license_name,
            license_url=license_url,
        )

        content_custom_export_meta.additional_properties = d
        return content_custom_export_meta

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
